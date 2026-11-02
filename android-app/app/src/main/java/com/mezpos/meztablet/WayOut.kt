package com.mezpos.meztablet

import java.net.URI
import java.net.URISyntaxException
import java.net.URLDecoder
import kotlin.math.abs

/*
 * The way out of mez (ADR-419): two fingers held still for three seconds,
 * then a manager's PIN, checked by the PC with the till's own check. The app
 * keeps no PIN; it asks the PC, with the device's token read from the till's
 * page, who may open the way out and whether a PIN does.
 */

/**
 * Two fingers held still for [holdMs], anywhere: the way out's gesture. Fed
 * each touch the activity sees, as the fingers down, their places (x and y,
 * in turn) and the time; answers when the hold falls due, while it is under
 * way. A finger moved past [slop], a third finger, or one lifted spoils it
 * until the fingers lift.
 */
class Hold(private val holdMs: Long = HOLD_MS, private val slop: Float) {
    private var since: Long? = null
    private var start = FloatArray(0)
    private var spoiled = false

    /** A touch, with [fingers] down at [places] at [at] ms: when the hold is due, or null. */
    fun touch(fingers: Int, places: FloatArray, at: Long): Long? {
        if (fingers == 0) {
            reset()
            return null
        }
        if (fingers != 2 || spoiled) {
            if (since != null || fingers > 2) spoiled = true
            since = null
            return null
        }
        val began = since
        if (began == null) {
            since = at
            start = places.copyOf()
            return at + holdMs
        }
        if (places.indices.any { it >= start.size || abs(places[it] - start[it]) > slop }) {
            spoiled = true
            since = null
            return null
        }
        return began + holdMs
    }

    /** Whether the hold is made by [now]. */
    fun made(now: Long): Boolean {
        val began = since ?: return false
        return !spoiled && now - began >= holdMs
    }

    /** Forgets the touch: made once, the hold is not made again until held again. */
    fun reset() {
        since = null
        spoiled = false
        start = FloatArray(0)
    }

    companion object {
        /** How long two fingers are held still. */
        const val HOLD_MS = 3_000L
    }
}

/** What the PC said of a PIN for the way out. */
enum class Opened(val why: String) {
    OPEN(""),
    WRONG_PIN("wrong-pin"),
    CANNOT("cannot"),
    LOCKED("locked"),
    NOT_PAIRED("not-paired"),
    NO_ANSWER("no-answer"),

    /** The address did not prove it is the PC holding the pairing (ADR-417): the PIN was not sent. */
    UNPROVEN("unproven"),
}

/**
 * What the PC's answer [status] and [body] to [staff]'s PIN for the way out
 * says. A yes names who opened it, and it must be [staff].
 */
fun openedOf(status: Int, body: String, staff: String): Opened {
    if (status == HTTP_OK) return if (OPENED_ID.find(body)?.groupValues?.get(1) == staff) Opened.OPEN else Opened.NO_ANSWER
    return when (CODE.find(body)?.groupValues?.get(1)) {
        "invalid_pin" -> Opened.WRONG_PIN
        "forbidden" -> Opened.CANNOT
        "locked_out" -> Opened.LOCKED
        "unauthenticated", "device_revoked" -> Opened.NOT_PAIRED
        else -> Opened.NO_ANSWER
    }
}

private val CODE = Regex("\"code\":\"([a-z_]+)\"")
private val OPENED_ID = Regex("\"opened_by\":\\{\"id\":\"([A-Za-z0-9_]{1,80})\"")
private const val HTTP_OK = 200

/** The name of who opened the way out, if the PC's answer [body] says it plainly (no escapes). */
fun openedByOf(body: String): String? = OPENED_BY.find(body)?.groupValues?.get(1)

private val OPENED_BY = Regex("\"opened_by\":\\{[^}]*\"name\":\"([^\"\\\\]{1,80})\"")

/** Who may open the way out, as the PC answered: its list, as JSON, for the
 * way out's page to read; null for anything else, or a list too long to
 * carry in the page's address.
 */
fun whoOf(status: Int, body: String): String? =
    body.takeIf { status == HTTP_OK && it.startsWith("{\"staff\":[") && it.length <= MAX_WHO }

// A venue's managers, with their names and roles, in a page's address.
private const val MAX_WHO = 16_000

/** The way out asked for: who, and their PIN, as JSON; null for an id or a PIN that is not one. */
fun wayOutBody(staff: String, pin: String): String? {
    if (!STAFF.matches(staff) || !PIN.matches(pin)) return null
    return "{\"authorised_by\":\"$staff\",\"pin\":\"$pin\"}"
}

private val STAFF = Regex("stf_[A-Za-z0-9]{1,64}")
private val PIN = Regex("[0-9]{1,12}")

/** Where the PC at [pc] serves the way out. */
fun wayOutUrl(pc: String): String? {
    if (AppPages.isOwn(pc)) return null
    val uri = try {
        URI(pc)
    } catch (e: URISyntaxException) {
        return null
    }
    if (uri.host.isNullOrEmpty() || uri.scheme == null) return null
    return URI(uri.scheme, null, uri.host, uri.port, WAY_OUT, null, null).toString()
}

private const val WAY_OUT = "/api/v1/devices/me/way-out"

/** The value of [name] in [url]'s query, decoded; null if it has none. */
fun queryOf(url: String, name: String): String? {
    val value = url.substringAfter('?', "").split('&')
        .firstOrNull { it.startsWith("$name=") }?.removePrefix("$name=") ?: return null
    return try {
        URLDecoder.decode(value, "UTF-8")
    } catch (e: IllegalArgumentException) {
        null
    }
}

/** The way out asked of the PC, on the caller's thread. */
object WayOutAsk {
    /** Who may open it at the PC at [pc], asked with the device's [token]; null if the PC does not say. */
    fun who(pc: String, token: String): String? {
        val url = wayOutUrl(pc) ?: return null
        val answer = Probe.fetch(url, bearer = token) ?: return null
        return whoOf(answer.status, answer.body)
    }

    /**
     * Whether [staff]'s [pin] opens it at the PC at [pc], asked with the
     * pairing [held], and the name of who opened it, if the PC said one
     * plainly. The address proves it holds the pairing first (ADR-417): a
     * device that took the PC's address is never sent the PIN.
     */
    fun open(pc: String, held: Held, staff: String, pin: String): Pair<Opened, String?> {
        val url = wayOutUrl(pc) ?: return Opened.NO_ANSWER to null
        val body = wayOutBody(staff, pin) ?: return Opened.WRONG_PIN to null
        if (!Proof.proves(pc, held)) return Opened.UNPROVEN to null
        val answer = Probe.fetch(url, bearer = held.token, post = body) ?: return Opened.NO_ANSWER to null
        return openedOf(answer.status, answer.body, staff) to openedByOf(answer.body)
    }
}

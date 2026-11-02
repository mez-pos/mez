package com.mezpos.meztablet

import java.net.URLEncoder

/**
 * The app's own pages: HTML in its assets (`start.html`, one section per
 * page), served by `WebViewAssetLoader` at an https origin that belongs to
 * the WebView and that nothing on the network answers for.
 */
object AppPages {
    const val ASSETS = "https://appassets.androidplatform.net/assets/"

    /** The PC did not answer; the page says where the app is looking. */
    const val UNREACHABLE = "unreachable"

    /** Android took the screen away twice in a short while ([revival]). */
    const val STOPPED = "stopped"

    /** Asking an address whether mez answers there, before keeping it. */
    const val CHECKING = "checking"

    /** The camera's picture under it: the page's words and Cancel over it. */
    const val SCANNING = "scanning"

    /** The way out (ADR-419): who may open it, and the PIN; then what it offers. */
    const val WAY_OUT = "way-out"
    const val WAY_OUT_OPEN = "way-out-open"

    /** "Pair this tablet" when the camera is refused, or there is none. */
    const val CAMERA_REFUSED = "refused"
    const val CAMERA_NONE = "none"

    /** The address [start] opens: the PC's own, [mez], or the app's page for it. */
    fun address(start: Start, mez: String): String =
        start.page?.let(::page) ?: mez

    /**
     * "Can't reach the mez PC", for [failed], the PC's address that did not
     * answer. The page shows the PC's host and port, never the rest: a
     * pairing link carries its code in the fragment.
     */
    fun unreachable(failed: String, lock: Locking? = null): String = withLock(at(UNREACHABLE, failed), lock)

    /** "Looking for mez" at [address]'s host and port, while the app asks it. */
    fun checking(address: String): String = at(CHECKING, address)

    private fun at(id: String, address: String): String {
        val pc = place(address) ?: return page(id)
        return "${ASSETS}start.html?pc=${URLEncoder.encode(pc, "UTF-8")}#$id"
    }

    /**
     * "Pair this tablet" again, saying [why] the link was refused, with
     * [typed] back in the box to be mended.
     */
    fun pairRefused(why: Why, typed: String): String =
        "${ASSETS}start.html?why=${why.id}&typed=${URLEncoder.encode(typed, "UTF-8")}#${Start.PAIR.page}"

    /** "Pair this tablet", saying why the camera cannot scan: [why], [CAMERA_REFUSED] or [CAMERA_NONE]. */
    fun pairWithoutCamera(why: String): String = "${ASSETS}start.html?camera=$why#${Start.PAIR.page}"

    /**
     * The way out, listing [who] may open it (the PC's JSON, which the page
     * reads), saying [why] the last PIN did not, or why it cannot be asked,
     * with the one [chosen] for it still chosen.
     */
    fun wayOut(who: String?, why: String?, chosen: String? = null): String {
        val query = listOfNotNull(
            who?.let { "who=" + URLEncoder.encode(it, "UTF-8") },
            why?.let { "why=" + URLEncoder.encode(it, "UTF-8") },
            chosen?.let { "chosen=" + URLEncoder.encode(it, "UTF-8") },
        ).joinToString("&")
        return "${ASSETS}start.html${if (query.isEmpty()) "" else "?$query"}#$WAY_OUT"
    }

    /** The way out opened, by [by], and what it offers: more when the tablet is held by [lock]. */
    fun wayOutOpen(by: String, lock: Locking? = null): String =
        withLock("${ASSETS}start.html?by=${URLEncoder.encode(by, "UTF-8")}#$WAY_OUT_OPEN", lock)

    // [page], saying how the tablet is held to mez (ADR-420): its links for a
    // locked tablet show.
    private fun withLock(page: String, lock: Locking?): String {
        if (lock == null) return page
        val address = page.substringBefore('#')
        val id = page.substringAfter('#', "")
        val joined = "$address${if ('?' in address) "&" else "?"}lock=${lock.id}"
        return if (id.isEmpty()) joined else "$joined#$id"
    }

    /** The app's page [id], one `<section id>` of `start.html`. */
    fun page(id: String): String = "${ASSETS}start.html#$id"

    /** Whether [url] is one of the app's own pages. */
    fun isOwn(url: String?): Boolean = url != null && url.startsWith(ASSETS)
}

/**
 * What the app's own pages ask the app for, each by a link `mez-app:<name>`;
 * one that [takesQuery] carries what was typed after a `?`.
 */
enum class Action(val link: String, val takesQuery: Boolean = false) {
    ASK_LOCAL_NETWORK("mez-app:ask"),
    OPEN_SETTINGS("mez-app:settings"),

    /** Try the PC again now, from "Can't reach the mez PC" or "stopped". */
    RETRY("mez-app:retry"),

    /** The PC's address typed on "Pair this tablet": `mez-app:address?typed=...`. */
    ADDRESS("mez-app:address", takesQuery = true),

    /** Forget the PC's address and pair again, from "Can't reach the mez PC". */
    PAIR_AGAIN("mez-app:pair-again"),

    /** The camera, from "Pair this tablet". */
    SCAN("mez-app:scan"),

    /** Back to "Pair this tablet" from the camera. */
    CANCEL_SCAN("mez-app:cancel-scan"),

    /** The way out (ADR-419): a PIN for it, `mez-app:way-out-open?staff=...&pin=...`. */
    WAY_OUT_OPEN("mez-app:way-out-open", takesQuery = true),

    /** Back to mez from the way out, open or not. */
    WAY_OUT_BACK("mez-app:way-out-back"),

    /** What the way out offers, once the PC has said yes: Android's settings, or pairing again. */
    WAY_OUT_SETTINGS("mez-app:way-out-settings"),
    WAY_OUT_PAIR_AGAIN("mez-app:way-out-pair-again"),

    /** The way out open on a locked tablet (ADR-420): the lock lifted until mez is back in front. */
    WAY_OUT_LEAVE("mez-app:way-out-leave"),

    /** The way out open on a kiosk: an ordinary tablet again, mez no longer its device owner. */
    WAY_OUT_RELEASE("mez-app:way-out-release"),

    /** Android's Wi-Fi settings, from "Can't reach the mez PC" on a kiosk, inside the lock. */
    WIFI("mez-app:wifi"),
    ;

    companion object {
        const val SCHEME = "mez-app:"

        /**
         * The action [url] asks for, or null. Only the app's own pages ask:
         * a page from the network, [from] anywhere else, asks for nothing.
         */
        fun of(url: String, from: String?): Action? {
            if (!AppPages.isOwn(from)) return null
            return entries.firstOrNull { it.link == url || (it.takesQuery && url.startsWith("${it.link}?")) }
        }
    }
}

/** What was typed in [url], `mez-app:address?typed=...`, decoded; null if nothing was. */
fun typedOf(url: String): String? = queryOf(url, "typed")

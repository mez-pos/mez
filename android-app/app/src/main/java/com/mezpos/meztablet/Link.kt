package com.mezpos.meztablet

import java.net.URI
import java.net.URISyntaxException

/** The screens of mez's a tablet opens, at the paths the server's `appPaths` names. */
enum class App(val path: String) {
    TILL("/"),
    KITCHEN("/kitchen"),
}

/** What the app makes of a pairing link, scanned or typed. */
sealed interface Pairing {
    /**
     * A link the app takes. [pc] is the PC's address and the app's path,
     * which the app keeps; [open] is the same with the pairing code, if the
     * link had one, which the app opens once and never keeps.
     */
    data class Link(val pc: String, val open: String, val app: App) : Pairing

    /** A link the app refuses, and [why], which the page puts in words. */
    data class Refused(val why: Why) : Pairing
}

/** Why a link was refused: each is a `data-why` paragraph of `start.html`. */
enum class Why(val id: String) {
    /** Not an address at all, or not one a browser opens. */
    NOT_AN_ADDRESS("not-an-address"),

    /** An address, but not a PC on the venue's network. */
    NOT_ON_THE_NETWORK("not-on-the-network"),

    /** The back office's address: a PC's work, not a tablet's. */
    BACK_OFFICE("back-office"),

    /** On the network, but not the till's or the kitchen display's. */
    NOT_AN_APP("not-an-app"),

    /** A fragment that is not a pairing code. */
    NOT_A_CODE("not-a-code"),

    /**
     * Read well, but nothing there answers as mez: another device, a PC that
     * is off, or https where mez serves http. Found by asking, not by reading.
     */
    NO_MEZ("no-mez"),
}

/** Longer than any address the Devices page shows, with its code. */
const val LINK_MAX = 200

/**
 * Reads [text], a link from the Devices page's QR (ADR-367) or an address
 * typed as the PC shows it, such as `192.168.1.20:7777/kitchen`. It takes
 * http or https, a private IPv4 address or `mez.local` (ADR-379), any port,
 * the till or the kitchen display, and an optional `#code=XXXX-XXXX`;
 * nothing else. No name is looked up: a check that asked the network
 * would answer differently offline.
 */
fun readLink(text: String): Pairing {
    val typed = text.trim()
    if (typed.isEmpty() || typed.length > LINK_MAX) return Pairing.Refused(Why.NOT_AN_ADDRESS)
    val written = if (typed.contains("://")) typed else "http://$typed"
    val uri = try {
        URI(written)
    } catch (e: URISyntaxException) {
        return Pairing.Refused(Why.NOT_AN_ADDRESS)
    }
    // java.net.URI reads no host in what is not one, as 192.168.1 or a part
    // over 255: not an address at all, rather than one off the network.
    val scheme = uri.scheme?.lowercase() ?: return Pairing.Refused(Why.NOT_AN_ADDRESS)
    val host = uri.host?.lowercase() ?: return Pairing.Refused(Why.NOT_AN_ADDRESS)
    val port = uri.port
    val shapeOk = (scheme == "http" || scheme == "https") && host.isNotEmpty() &&
        uri.rawUserInfo == null && uri.rawQuery == null && (port == -1 || port in 1..MAX_PORT)
    if (!shapeOk) return Pairing.Refused(Why.NOT_AN_ADDRESS)
    if (!onTheNetwork(host)) return Pairing.Refused(Why.NOT_ON_THE_NETWORK)

    val app = when (uri.rawPath.orEmpty().lowercase()) {
        "", "/" -> App.TILL
        "/kitchen", "/kitchen/" -> App.KITCHEN
        else -> return Pairing.Refused(
            if (uri.rawPath.startsWith("/backoffice", ignoreCase = true)) Why.BACK_OFFICE else Why.NOT_AN_APP,
        )
    }
    val code = when (val fragment = uri.rawFragment) {
        null, "" -> null
        else -> pairingCode(fragment) ?: return Pairing.Refused(Why.NOT_A_CODE)
    }

    val pc = "$scheme://$host${if (port == -1) "" else ":$port"}${app.path}"
    return Pairing.Link(pc = pc, open = if (code == null) pc else "$pc#code=$code", app = app)
}

private const val MAX_PORT = 65535

/**
 * Whether [host] is a PC on a venue's network: an address in 10/8,
 * 172.16/12 or 192.168/16, or `mez.local`. Not a public address, not a
 * name the internet answers for, not this tablet (127/8). A part with a
 * leading zero is refused: Chromium reads `020` as octal, another address.
 */
fun onTheNetwork(host: String): Boolean {
    if (host == "mez.local") return true
    val parts = host.split('.')
    val decimal = parts.size == 4 && parts.all { part ->
        part.length in 1..3 && part.all { it in '0'..'9' } && !(part.length > 1 && part.startsWith('0')) &&
            part.toInt() <= MAX_OCTET
    }
    if (!decimal) return false
    val (a, b) = parts.take(2).map(String::toInt)
    return a == 10 || (a == 172 && b in 16..31) || (a == 192 && b == 168)
}

private const val MAX_OCTET = 255

/**
 * The pairing code in [fragment], written as the web app writes one
 * (`shared/auth/format.ts`): `code=`, then four letters or digits, a hyphen
 * if it likes, and four more; uppercased, `XXXX-XXXX`. Null for anything
 * else in the fragment.
 */
private fun pairingCode(fragment: String): String? {
    val match = CODE.matchEntire(fragment) ?: return null
    val (first, second) = match.destructured
    return "$first-$second".uppercase()
}

private val CODE = Regex("code=([A-Za-z0-9]{4})-?([A-Za-z0-9]{4})")

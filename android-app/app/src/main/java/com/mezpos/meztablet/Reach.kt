package com.mezpos.meztablet

import android.webkit.WebViewClient
import java.net.URI
import java.net.URISyntaxException

/** The PC's health check (`internal/api/health.go`): 200 whenever mez is up. */
const val HEALTH = "/api/v1/health"

/**
 * The WebView's errors that mean the PC did not answer: no such address on
 * the network, no connection, no answer in time, a connection dropped. Not
 * a bad address, a refused scheme or a redirect loop, which trying again
 * does not mend: those keep WebView's own error page.
 */
private val NO_ANSWER = setOf(
    WebViewClient.ERROR_HOST_LOOKUP,
    WebViewClient.ERROR_CONNECT,
    WebViewClient.ERROR_TIMEOUT,
    WebViewClient.ERROR_IO,
)

/** Whether the WebView's [error], for the PC's page, means the PC did not answer. */
fun noAnswer(error: Int): Boolean = error in NO_ANSWER

/**
 * What to open again once the PC answers, for [failed], the address the
 * WebView reports. That address has lost its fragment, and the one the app
 * asked for, [asked], may carry the pairing code in it (ADR-367): the same
 * page, so the code goes with it.
 */
fun retryOf(failed: String, asked: String?): String =
    if (asked != null && asked.substringBefore('#') == failed.substringBefore('#')) asked else failed

/**
 * Whether [url] is on the PC at [pc]: the same scheme, host and port. A
 * new origin would be a new localStorage, without the device's pairing.
 */
fun samePc(url: String, pc: String): Boolean {
    val at = origin(url) ?: return false
    return at == origin(pc)
}

/**
 * Whether a page may take the WebView from [from] to [url], with the PC at
 * [pc]: to the PC's own pages, and to the app's pages from the app's pages.
 * Not to the internet, nor to another device on the network: the tablet is
 * the venue's till, its WebView holds the PC's pairing, and a link out would
 * leave a till showing someone else's page. Other schemes are WebView's to
 * refuse (no file or content access, no apps opened).
 */
fun mayOpen(url: String, from: String?, pc: String): Boolean = when {
    AppPages.isOwn(url) -> AppPages.isOwn(from)
    url.startsWith("http://", ignoreCase = true) || url.startsWith("https://", ignoreCase = true) -> samePc(url, pc)
    else -> true
}

/** The address the app asks to know mez is back at [url]'s PC, or null if [url] is no PC's. */
fun healthOf(url: String): String? {
    val pc = pc(url) ?: return null
    return URI(pc.scheme, null, pc.host, pc.port, HEALTH, null, null).toString()
}

/** The PC's host and port in [url], as the app's page shows them: `192.168.1.20:7777`. */
fun place(url: String): String? {
    val pc = pc(url) ?: return null
    return if (pc.port == -1) pc.host else "${pc.host}:${pc.port}"
}

private data class Origin(val scheme: String, val host: String, val port: Int)

private fun origin(url: String): Origin? {
    val pc = pc(url) ?: return null
    val scheme = pc.scheme.lowercase()
    val port = if (pc.port != -1) pc.port else if (scheme == "https") HTTPS_PORT else HTTP_PORT
    return Origin(scheme, pc.host.lowercase(), port)
}

private const val HTTP_PORT = 80
private const val HTTPS_PORT = 443

// java.net.URI, not android.net.Uri: the JVM's unit tests can run it. A
// host it cannot read (an underscore in a name) is no PC's: the build
// refuses such an address for -PmezUrl.
private fun pc(url: String): URI? {
    val uri = try {
        URI(url)
    } catch (e: URISyntaxException) {
        return null
    }
    val web = uri.scheme.equals("http", ignoreCase = true) || uri.scheme.equals("https", ignoreCase = true)
    return uri.takeIf { web && !it.host.isNullOrEmpty() }
}

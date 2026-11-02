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

    /** The address [start] opens: the PC's own, [mez], or the app's page for it. */
    fun address(start: Start, mez: String): String =
        start.page?.let(::page) ?: mez

    /**
     * "Can't reach the mez PC", for [failed], the PC's address that did not
     * answer. The page shows the PC's host and port, never the rest: a
     * pairing link carries its code in the fragment.
     */
    fun unreachable(failed: String): String {
        val pc = place(failed) ?: return page(UNREACHABLE)
        return "${ASSETS}start.html?pc=${URLEncoder.encode(pc, "UTF-8")}#$UNREACHABLE"
    }

    /** The app's page [id], one `<section id>` of `start.html`. */
    fun page(id: String): String = "${ASSETS}start.html#$id"

    /** Whether [url] is one of the app's own pages. */
    fun isOwn(url: String?): Boolean = url != null && url.startsWith(ASSETS)
}

/** What the app's own pages ask the app for, each by a link `mez-app:<name>`. */
enum class Action(val link: String) {
    ASK_LOCAL_NETWORK("mez-app:ask"),
    OPEN_SETTINGS("mez-app:settings"),

    /** Try the PC again now, from "Can't reach the mez PC" or "stopped". */
    RETRY("mez-app:retry"),
    ;

    companion object {
        const val SCHEME = "mez-app:"

        /**
         * The action [url] asks for, or null. Only the app's own pages ask:
         * a page from the network, [from] anywhere else, asks for nothing.
         */
        fun of(url: String, from: String?): Action? =
            if (AppPages.isOwn(from)) entries.firstOrNull { it.link == url } else null
    }
}

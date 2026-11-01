package com.mezpos.meztablet

/**
 * The app's own pages: HTML in its assets (`start.html`, one section per
 * [Start] page), served by `WebViewAssetLoader` at an https origin that
 * belongs to the WebView and that nothing on the network answers for.
 */
object AppPages {
    const val ASSETS = "https://appassets.androidplatform.net/assets/"

    /** The address [start] opens: the PC's own, [mez], or the app's page for it. */
    fun address(start: Start, mez: String): String =
        start.page?.let { "${ASSETS}start.html#$it" } ?: mez

    /** Whether [url] is one of the app's own pages. */
    fun isOwn(url: String?): Boolean = url != null && url.startsWith(ASSETS)
}

/** What the app's own pages ask the app for, each by a link `mez-app:<name>`. */
enum class Action(val link: String) {
    ASK_LOCAL_NETWORK("mez-app:ask"),
    OPEN_SETTINGS("mez-app:settings"),
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

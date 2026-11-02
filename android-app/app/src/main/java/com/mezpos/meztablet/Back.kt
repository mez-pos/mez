package com.mezpos.meztablet

/**
 * Whether Back goes back in the WebView, as Back does in a browser: from
 * [current], one of the PC's pages, when WebView can go back ([canGoBack]),
 * and none of the app's own pages is in the history [behind] it. Never from
 * the first screen, where Android's Back would leave the app.
 *
 * WebView's Back skips the entries a page added without a tap, as Chrome
 * does, so it may land further back than the entry before [current]: any
 * of the app's pages behind ("Can't reach the mez PC", the local network's)
 * keeps Back where it is. The app clears the history when it opens the PC's
 * page, so there should be none.
 */
fun goesBack(current: String?, behind: List<String?>, canGoBack: Boolean): Boolean =
    canGoBack && current != null && !AppPages.isOwn(current) && behind.none(AppPages::isOwn)

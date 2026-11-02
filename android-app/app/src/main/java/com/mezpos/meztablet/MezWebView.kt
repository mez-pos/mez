package com.mezpos.meztablet

import android.annotation.SuppressLint
import android.content.Context
import android.view.View
import android.webkit.WebChromeClient
import android.webkit.WebView
import android.webkit.WebViewClient

/**
 * The app's one WebView, set as the till and the kitchen display need it
 * (ADR-413). What is not set here is WebView's default, chosen as well:
 * one window (a link to a new tab opens in this one), no downloads, the
 * HTTP cache as the server's headers say, no mixed content, third-party cookies off, the
 * light theme the screens draw (no forced dark), WebView's own user agent.
 */
@SuppressLint("SetJavaScriptEnabled")
fun Context.mezWebView(client: WebViewClient): WebView = WebView(this).apply {
    setBackgroundColor(getColor(R.color.parchment))
    // PINs are typed on these screens, the way out's included: Android's
    // autofill is not to offer to keep one.
    importantForAutofill = View.IMPORTANT_FOR_AUTOFILL_NO_EXCLUDE_DESCENDANTS
    // A till is an app, not a page: the whole screen does not stretch when
    // a list is pulled past its end. Lists inside the page still scroll.
    overScrollMode = View.OVER_SCROLL_NEVER
    webViewClient = client
    // WebView's own dialogs for alert() and confirm(). With no chrome client
    // WebView answers them unseen, confirm() as Cancel: the till's "Leave
    // anyway?" over items still being added would refuse without asking.
    // No file chooser: the back office's CSV import and item photo are a
    // PC's work, and no camera or microphone for a page (ADR-413).
    webChromeClient = WebChromeClient()
    settings.apply {
        // The screens are a JavaScript app, and the device's pairing is
        // kept in the origin's localStorage (ADR-338).
        javaScriptEnabled = true
        domStorageEnabled = true

        // A sound from an <audio> or <video> element plays with nobody
        // touching the screen. The kitchen chime is Web Audio, which WebView
        // does not hold back for a tap either way (ADR-413).
        mediaPlaybackRequiresUserGesture = false

        // The screens are laid out at the tablet's own size (ADR-381): the
        // tablet's font size and a pinch must not lay them out again.
        textZoom = 100
        setSupportZoom(false)
        builtInZoomControls = false
        displayZoomControls = false

        // Nothing on the tablet is the screens' to read: not its files,
        // not other apps' content, not where it is.
        allowFileAccess = false
        allowContentAccess = false
        setGeolocationEnabled(false)

        // The WebView opens the venue's PC and the app's own pages, never the
        // internet, and Safe Browsing's lists come from Google: offline, the
        // check has nothing to say about a PC on the venue's wifi.
        safeBrowsingEnabled = false
    }
}

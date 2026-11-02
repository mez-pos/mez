package com.mezpos.meztablet

/** What the app does when Android takes the WebView's renderer away. */
enum class Revival {
    /** A new WebView, at the page that was showing. */
    RELOAD,

    /**
     * A new WebView at the app's "stopped" page, until someone taps Try
     * again: a page that takes its renderer down each time it loads would
     * otherwise be loaded over and over, as fast as the tablet can.
     */
    STOP,
}

/** Gone again within this long of the last time is a page that keeps going. */
const val GONE_AGAIN_MS = 60_000L

/**
 * What to do when the renderer is gone at [now], having last gone at
 * [lastGone] (both on the same clock, `SystemClock.elapsedRealtime`), or
 * never. Once is Android freeing memory, or a crash: the page comes back.
 * Twice inside [GONE_AGAIN_MS] stops for a person to choose.
 */
fun revival(now: Long, lastGone: Long?): Revival =
    if (lastGone != null && now - lastGone < GONE_AGAIN_MS) Revival.STOP else Revival.RELOAD

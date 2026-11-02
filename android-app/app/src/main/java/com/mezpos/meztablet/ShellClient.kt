package com.mezpos.meztablet

import android.util.Log
import android.webkit.RenderProcessGoneDetail
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.webkit.WebViewAssetLoader

/** What the WebView tells the activity, through [ShellClient]. */
interface Shell {
    /** The PC's address: its pages, and no others, are the WebView's to open. */
    val pc: String

    /** One of the app's own pages asked for [action], by [link]. */
    fun act(action: Action, link: String)

    /** [url], one of the PC's pages, did not load in [view]: the PC did not answer. */
    fun unreachable(view: WebView, url: String)

    /** [view] is at [url] now: a load, a reload, or the page's own history. */
    fun visited(view: WebView, url: String)

    /** Android took [view]'s renderer away: [view] is dead, and must go. */
    fun gone(view: WebView, crashed: Boolean)
}

/**
 * Serves the app's own pages from its assets, turns their `mez-app:` links
 * into [Action]s, keeps the WebView on the PC's pages and the app's
 * ([mayOpen]), and tells [shell] when the PC does not answer and when the
 * renderer is gone.
 */
class ShellClient(
    private val assets: WebViewAssetLoader,
    private val shell: Shell,
) : WebViewClient() {

    override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? =
        assets.shouldInterceptRequest(request.url)

    override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
        val url = request.url.toString()
        if (!url.startsWith(Action.SCHEME)) {
            if (!request.isForMainFrame || mayOpen(url, from = view.url, pc = shell.pc)) return false
            // Only the host: the rest of an address may carry a code.
            Log.w(TAG, "refused to leave the PC for ${request.url.host}")
            return true
        }
        val action = Action.of(url, from = view.url)
        if (action == null) {
            // The page's own address is not logged: a pairing link carries its
            // code. The link is cut short: a page on the network wrote it.
            Log.w(TAG, "refused ${url.take(LOGGED_LINK)}: not asked by one of the app's own pages")
        } else {
            shell.act(action, url)
        }
        return true
    }

    override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
        val url = request.url.toString()
        if (!request.isForMainFrame || !samePc(url, shell.pc)) return
        if (noAnswer(error.errorCode)) {
            shell.unreachable(view, url)
        } else {
            // WebView's own error page stays: trying again would not mend it.
            Log.w(TAG, "the PC's page did not load: ${error.errorCode} ${error.description}")
        }
    }

    override fun doUpdateVisitedHistory(view: WebView, url: String, isReload: Boolean) {
        shell.visited(view, url)
    }

    // True: the app, not Android, deals with it, and lives on.
    override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
        shell.gone(view, crashed = detail.didCrash())
        return true
    }

    private companion object {
        const val TAG = "mez"
        const val LOGGED_LINK = 64
    }
}

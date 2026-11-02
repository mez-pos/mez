package com.mezpos.meztablet

import android.content.ActivityNotFoundException
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.os.SystemClock
import android.provider.Settings
import android.util.Log
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import android.webkit.WebView
import android.widget.FrameLayout
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.addCallback
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.core.net.toUri
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.Lifecycle
import androidx.webkit.WebViewAssetLoader

/**
 * The app's one activity and its one WebView (ADR-409). It shows the mez PC's
 * screens, as a browser would, full screen and awake (ADR-413); until Android
 * lets the app reach the PC, or while the PC does not answer, it shows the
 * app's own page that says why.
 */
class MainActivity : ComponentActivity() {

    private lateinit var frame: FrameLayout
    private lateinit var web: WebView
    private var showing: Start? = null
    private val prefs by lazy { getSharedPreferences(PREFS, MODE_PRIVATE) }

    // True from Continue to Android's answer. A second tap meanwhile would be
    // refused by Android with an empty answer, read as "refused past asking",
    // and show the Settings page behind the prompt.
    private var asking = false

    private val askLocalNetwork =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) {
            asking = false
            show(where())
        }

    /** The PC's page an app's page stands in for, and which page: what Try again opens. */
    private class Away(val url: String, val page: String)

    private var away: Away? = null

    // The last of the PC's pages the WebView was at, and the last the app
    // opened: what a new WebView reopens, and a pairing code a retry keeps.
    // A code already spent is harmless: only the pairing screen reads it,
    // and a paired device never shows that screen.
    private var lastMez: String? = null
    private var opened: String? = null

    // Set when the app opens one of the PC's pages; the history is cleared
    // once it is there, so none of the app's own pages is left behind the
    // till for Back to land on.
    private var forget = false

    // When Android last took the renderer away in front, for revival(); and
    // whether one taken in the background is still to be brought back.
    private var lastGone: Long? = null
    private var reviveOnStart = false

    // The stand-in the probe asks for: its answer opens that one, and no
    // stand-in that took its place before the answer reached this thread.
    private var probing: Away? = null
    private val probe = Probe { runOnUiThread(::answered) }

    private val shell = object : Shell {
        override val pc: String get() = BuildConfig.MEZ_URL
        override fun act(action: Action) = this@MainActivity.act(action)
        override fun unreachable(view: WebView, url: String) {
            if (view === web) this@MainActivity.unreachable(url)
        }
        override fun visited(view: WebView, url: String) {
            if (view !== web || !samePc(url, pc)) return
            lastMez = url
            if (forget) {
                forget = false
                web.clearHistory()
            }
        }
        override fun gone(view: WebView, crashed: Boolean) = this@MainActivity.gone(view, crashed)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        val ground = getColor(R.color.parchment)
        enableEdgeToEdge(SystemBarStyle.light(ground, ground), SystemBarStyle.light(ground, ground))
        super.onCreate(savedInstanceState)
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
        // A till is looked at more than touched, for minutes at a time in
        // service: the screen stays on while mez is in front.
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        web = newWebView()
        frame = FrameLayout(this).apply { addView(web) }
        setContentView(frame)
        keepClearOfCutoutAndKeyboard(frame)
        onBackPressedDispatcher.addCallback(this) { back() }
    }

    override fun onStart() {
        super.onStart()
        if (reviveOnStart) {
            reviveOnStart = false
            reopen(Revival.RELOAD)
        }
        // Asking stops while the app is not in front; it starts again at once.
        if (away?.page == AppPages.UNREACHABLE) tryAgain(after = 0)
    }

    // On first run, and on coming back from Android's prompt or its Settings,
    // where the local network may have been allowed since.
    override fun onResume() {
        super.onResume()
        hideSystemBars()
        if (!asking) show(where())
    }

    // A prompt or a dialog over the app can bring the bars back.
    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus) hideSystemBars()
    }

    override fun onStop() {
        probe.stop()
        super.onStop()
    }

    override fun onDestroy() {
        probe.close()
        // Not made if the WebView itself failed to load (one being updated).
        if (::web.isInitialized) {
            (web.parent as? ViewGroup)?.removeView(web)
            web.destroy()
        }
        super.onDestroy()
    }

    private fun where(): Start = start(
        address = BuildConfig.MEZ_URL,
        sdk = Build.VERSION.SDK_INT,
        granted = ContextCompat.checkSelfPermission(this, LOCAL_NETWORK) ==
            PackageManager.PERMISSION_GRANTED,
        askedBefore = prefs.getBoolean(ASKED, false),
        androidWillAsk = shouldShowRequestPermissionRationale(LOCAL_NETWORK),
    )

    private fun show(next: Start) {
        if (next == showing) return
        showing = next
        open(AppPages.address(next, mez = BuildConfig.MEZ_URL))
    }

    // The app's own way to a page; a stand-in it replaces stops asking.
    private fun open(url: String) {
        away = null
        probe.stop()
        if (!AppPages.isOwn(url)) {
            opened = url
            forget = true
        }
        web.loadUrl(url)
    }

    private fun act(action: Action) {
        when (action) {
            Action.ASK_LOCAL_NETWORK -> {
                if (asking) return
                asking = true
                prefs.edit { putBoolean(ASKED, true) }
                askLocalNetwork.launch(LOCAL_NETWORK)
            }
            Action.OPEN_SETTINGS -> openSettings()
            Action.RETRY -> retry()
        }
    }

    // A tablet locked down by its owner may have no Settings to open; the
    // page stays, and says where the permission is.
    private fun openSettings() {
        val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS, "package:$packageName".toUri())
        try {
            startActivity(intent)
        } catch (e: ActivityNotFoundException) {
            Log.w(TAG, "no Settings to open for the local network permission", e)
        }
    }

    // "Can't reach the mez PC" over the page that did not load, asking the PC
    // by itself until it answers.
    private fun unreachable(failed: String) {
        standIn(Away(retryOf(failed, asked = opened), AppPages.UNREACHABLE))
        tryAgain(after = Probe.RETRY_EVERY_MS)
    }

    private fun standIn(page: Away) {
        away = page
        web.loadUrl(
            if (page.page == AppPages.UNREACHABLE) AppPages.unreachable(page.url) else AppPages.page(page.page),
        )
    }

    // Try again: "Can't reach the mez PC" asks the PC first, so WebView's
    // error page never shows; "The screen stopped" opens the page at once,
    // and a PC that does not answer then says so.
    private fun retry() {
        val page = away ?: return
        if (page.page == AppPages.UNREACHABLE) tryAgain(after = 0) else open(page.url)
    }

    private fun tryAgain(after: Long) {
        val page = away ?: return
        val health = healthOf(page.url) ?: return
        probing = page
        probe.start(health, after)
    }

    // The PC answered: back to the page it stood in for.
    private fun answered() {
        if (isDestroyed) return
        val page = away ?: return
        if (page !== probing) return
        open(page.url)
    }

    private fun back() {
        val history = web.copyBackForwardList()
        val behind = (0 until history.currentIndex).map { history.getItemAtIndex(it)?.url }
        if (goesBack(history.currentItem?.url, behind, web.canGoBack())) web.goBack()
    }

    // The renderer died or Android took it for memory: the dead WebView goes,
    // and a new one opens where the old one was (ADR-413).
    private fun gone(dead: WebView, crashed: Boolean) {
        if (dead !== web) return
        Log.w(TAG, if (crashed) "the screen's renderer crashed" else "Android took the screen's renderer for memory")
        frame.removeView(dead)
        dead.destroy()
        web = newWebView()
        frame.addView(web, 0)

        // Behind another app, Android takes a renderer for memory as a matter
        // of course: the page comes back with the app, and does not count
        // towards "The screen stopped".
        if (!lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)) {
            reviveOnStart = true
            return
        }
        val now = SystemClock.elapsedRealtime()
        reopen(revival(now, lastGone))
        lastGone = now
    }

    // The new WebView opens what the old one showed, or "The screen stopped".
    private fun reopen(next: Revival) {
        val standing = away
        when {
            // The app's own pages are a few kilobytes: they come back as they were.
            showing != Start.MEZ -> {
                showing = null
                show(where())
            }
            next == Revival.STOP -> {
                probe.stop()
                probing = null
                standIn(Away(standing?.url ?: lastMez ?: opened ?: BuildConfig.MEZ_URL, AppPages.STOPPED))
            }
            standing != null -> standIn(standing)
            else -> open(lastMez ?: opened ?: BuildConfig.MEZ_URL)
        }
    }

    private fun newWebView(): WebView {
        val assets = WebViewAssetLoader.Builder()
            .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(this))
            .build()
        return mezWebView(ShellClient(assets, shell))
    }

    // Full screen: the bars come back for a moment with a swipe from the edge,
    // over the screen, and go again.
    private fun hideSystemBars() {
        WindowCompat.getInsetsController(window, window.decorView).apply {
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
            hide(WindowInsetsCompat.Type.systemBars())
        }
    }

    // Edge to edge, the window no longer shrinks for the keyboard, and the
    // screens keep clear of a camera's cutout. The bars count while they show,
    // on a tablet where Android will not hide them.
    private fun keepClearOfCutoutAndKeyboard(frame: View) {
        ViewCompat.setOnApplyWindowInsetsListener(frame) { view, insets ->
            val bars = insets.getInsets(
                WindowInsetsCompat.Type.systemBars() or
                    WindowInsetsCompat.Type.displayCutout() or
                    WindowInsetsCompat.Type.ime(),
            )
            view.setPadding(bars.left, bars.top, bars.right, bars.bottom)
            WindowInsetsCompat.CONSUMED
        }
    }

    private companion object {
        const val TAG = "mez"
        const val PREFS = "mez"
        const val ASKED = "asked-local-network"
    }
}

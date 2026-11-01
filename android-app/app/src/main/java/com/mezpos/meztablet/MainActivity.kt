package com.mezpos.meztablet

import android.annotation.SuppressLint
import android.content.ActivityNotFoundException
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import android.util.Log
import android.view.View
import android.view.ViewGroup
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import androidx.core.content.edit
import androidx.core.net.toUri
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.webkit.WebViewAssetLoader

/**
 * The app's one activity and its one WebView (ADR-409). It shows the mez PC's
 * screens, as a browser would; until Android lets the app reach the PC, it
 * shows the app's own page that says why, and asks.
 */
class MainActivity : ComponentActivity() {

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

    override fun onCreate(savedInstanceState: Bundle?) {
        val ground = getColor(R.color.parchment)
        enableEdgeToEdge(SystemBarStyle.light(ground, ground), SystemBarStyle.light(ground, ground))
        super.onCreate(savedInstanceState)
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG)
        web = newWebView()
        val frame = FrameLayout(this).apply { addView(web) }
        setContentView(frame)
        keepClearOfSystemBars(frame)
    }

    // On first run, and on coming back from Android's prompt or its Settings,
    // where the local network may have been allowed since.
    override fun onResume() {
        super.onResume()
        if (!asking) show(where())
    }

    override fun onDestroy() {
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
        web.loadUrl(AppPages.address(next, mez = BuildConfig.MEZ_URL))
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

    // Task 3 chooses the rest of the settings and writes them down. These two
    // the till cannot run without: it is a JavaScript app, and the device's
    // pairing is kept in the origin's localStorage (ADR-338).
    @SuppressLint("SetJavaScriptEnabled")
    private fun newWebView(): WebView {
        val assets = WebViewAssetLoader.Builder()
            .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(this))
            .build()
        return WebView(this).apply {
            setBackgroundColor(getColor(R.color.parchment))
            settings.javaScriptEnabled = true
            settings.domStorageEnabled = true
            webViewClient = ShellClient(assets, ::act)
        }
    }

    // Task 3 hides the system bars. Until then the screens sit between them,
    // and above the keyboard: edge to edge, the window no longer shrinks for it.
    private fun keepClearOfSystemBars(frame: View) {
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

/**
 * Serves the app's own pages from its assets, and turns their `mez-app:`
 * links into [Action]s. Every other address loads in the WebView as it is.
 *
 * A renderer that dies still takes the app with it: bringing the WebView back
 * (`onRenderProcessGone`) is Phase 18's Task 3, seen on a tablet there.
 */
@SuppressLint("MissingOnRenderProcessGone")
private class ShellClient(
    private val assets: WebViewAssetLoader,
    private val act: (Action) -> Unit,
) : WebViewClient() {

    override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? =
        assets.shouldInterceptRequest(request.url)

    override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
        val url = request.url.toString()
        if (!url.startsWith(Action.SCHEME)) return false
        val action = Action.of(url, from = view.url)
        if (action == null) {
            // The page's own address is not logged: a pairing link carries its
            // code. The link is cut short: a page on the network wrote it.
            Log.w(TAG, "refused ${url.take(LOGGED_LINK)}: not asked by one of the app's own pages")
        } else {
            act(action)
        }
        return true
    }

    private companion object {
        const val TAG = "mez"
        const val LOGGED_LINK = 64
    }
}

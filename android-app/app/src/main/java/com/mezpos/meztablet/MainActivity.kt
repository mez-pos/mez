package com.mezpos.meztablet

import android.Manifest
import android.content.ActivityNotFoundException
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color
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

    // Under the WebView: the camera's picture while "Pair this tablet" scans.
    private lateinit var camera: FrameLayout
    private lateinit var scanner: Scanner
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

    // True from Scan to Android's answer, as [asking] is for the local
    // network: a second request meanwhile is answered "no" by Android.
    private var askingCamera = false

    private val askCamera =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
            askingCamera = false
            if (showing != Start.PAIR) return@registerForActivityResult
            if (granted) startScan() else load(AppPages.pairWithoutCamera(AppPages.CAMERA_REFUSED))
        }

    // The PC's address and the app's path, kept once mez has answered there
    // (ADR-414); empty until then.
    private val pc: String
        get() = prefs.getString(PC, null).orEmpty()

    // Where the device's pairing is: the PC's address, or after "Pair again"
    // the one it had (ADR-416). Before that ADR only the PC's address was kept.
    private val held: String
        get() = prefs.getString(HELD, null) ?: pc

    // Whether this run has asked the PC which venue it serves, for a PC kept
    // before health named it: once is enough.
    private var askedVenue = false

    // A link read well ("Pair this tablet", readLink()) and not yet asked:
    // held, with its pairing code, until mez answers at it, then kept without
    // the code and opened with it once. What was typed, for a refusal.
    private var candidate: Pairing.Link? = null
    private var candidateText = ""

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
    private val probe = Probe { mez -> runOnUiThread { answered(mez) } }

    /**
     * A carry under way (ADR-416): the pairing being read at [from], the
     * address that holds it ([write] null), or written at [link]'s ([write]
     * the script that writes it). [page] is the app's empty page that step is
     * served.
     */
    private data class Carrying(
        val from: String,
        val link: Pairing.Link,
        val venue: String,
        val page: String,
        val write: String?,
    )

    // Volatile: ShellClient asks which page to serve off the main thread.
    @Volatile
    private var carry: Carrying? = null

    private val shell = object : Shell {
        override val pc: String get() = this@MainActivity.pc
        override fun act(action: Action, link: String) = this@MainActivity.act(action, link)
        override fun unreachable(view: WebView, url: String) {
            if (view === web) this@MainActivity.unreachable(url)
        }
        override fun visited(view: WebView, url: String) {
            if (view !== web || isCarryPage(url) || !samePc(url, pc)) return
            lastMez = url
            if (forget) {
                forget = false
                web.clearHistory()
            }
            askVenue()
        }
        override fun gone(view: WebView, crashed: Boolean) = this@MainActivity.gone(view, crashed)
        override val carrying: String? get() = carry?.page
        override fun finished(view: WebView, url: String) {
            if (view === web) this@MainActivity.finished(url)
        }
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
        camera = FrameLayout(this).apply { visibility = View.GONE }
        frame = FrameLayout(this).apply {
            addView(camera)
            addView(web)
        }
        scanner = Scanner(this, camera, found = ::scanned, failed = {
            load(AppPages.pairWithoutCamera(AppPages.CAMERA_NONE))
        })
        setContentView(frame)
        keepClearOfCutoutAndKeyboard(frame)
        onBackPressedDispatcher.addCallback(this) { back() }
        seedOnce()
    }

    // A debug build given an address (-PmezUrl) starts from it once, as if it
    // had been typed: after "Pair again" it does not come back.
    private fun seedOnce() {
        if (BuildConfig.MEZ_URL.isEmpty() || prefs.getBoolean(SEEDED, false)) return
        prefs.edit { putBoolean(SEEDED, true) }
        if (pc.isNotEmpty()) return
        val link = readLink(BuildConfig.MEZ_URL) as? Pairing.Link ?: return
        candidate = link
        candidateText = BuildConfig.MEZ_URL
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
        // A scan ends when the app goes behind another: Scan starts it again.
        if (::scanner.isInitialized && scanner.isRunning) load(AppPages.page(Start.PAIR.page.orEmpty()))
        super.onStop()
    }

    override fun onDestroy() {
        probe.close()
        if (::scanner.isInitialized) scanner.stop()
        // Not made if the WebView itself failed to load (one being updated).
        if (::web.isInitialized) {
            (web.parent as? ViewGroup)?.removeView(web)
            web.destroy()
        }
        super.onDestroy()
    }

    private fun where(): Start = start(
        address = pc.ifEmpty { candidate?.pc ?: carry?.link?.pc.orEmpty() },
        sdk = Build.VERSION.SDK_INT,
        granted = ContextCompat.checkSelfPermission(this, LOCAL_NETWORK) ==
            PackageManager.PERMISSION_GRANTED,
        askedBefore = prefs.getBoolean(ASKED, false),
        androidWillAsk = shouldShowRequestPermissionRationale(LOCAL_NETWORK),
    )

    private fun show(next: Start) {
        if (next == showing) return
        showing = next
        val link = candidate
        if (next == Start.MEZ && link != null) check(link) else open(AppPages.address(next, mez = pc))
    }

    // Asks the address whether mez answers there before keeping it: a router
    // or a printer at a private address answers too, and once kept, the
    // WebView would be held to it (ADR-413) with no way back.
    private fun check(link: Pairing.Link) {
        val health = healthOf(link.pc)
        if (health == null) {
            checked(link, mez = null)
            return
        }
        load(AppPages.checking(link.pc))
        probe.once(health) { mez -> runOnUiThread { checked(link, mez) } }
    }

    // mez answered at the link, or nothing did. The same venue's mez at
    // another address than the one holding the pairing takes the pairing
    // with it first (ADR-416).
    private fun checked(link: Pairing.Link, mez: Mez?) {
        if (isDestroyed || candidate !== link) return
        candidate = null
        val venue = mez?.venue
        when {
            mez == null -> {
                showing = Start.PAIR
                load(AppPages.pairRefused(Why.NO_MEZ, candidateText))
            }
            venue != null && carries(held, prefs.getString(VENUE, null), link.pc, venue) ->
                carryFrom(held, link, venue)
            else -> keep(link, venue)
        }
    }

    // The address kept, with the venue its mez named, and opened with its
    // code once. The pairing is held there from now on, unless a carry that
    // was not made leaves it [held] where it was.
    private fun keep(link: Pairing.Link, venue: String?, held: String = link.pc) {
        prefs.edit {
            putString(PC, link.pc)
            putString(HELD, held)
            if (venue == null) remove(VENUE) else putString(VENUE, venue)
        }
        open(link.open)
    }

    // The carry's first step: the app's empty page at the address holding the
    // pairing, where finished() reads it. A carry that has not ended in a few
    // seconds is given up: the new address opens as without one.
    private fun carryFrom(from: String, link: Pairing.Link, venue: String) {
        val page = carryPage(from) ?: return keep(link, venue)
        val first = Carrying(from, link, venue, page, write = null)
        carry = first
        load(page)
        frame.postDelayed({
            if (!isDestroyed && carry?.link === link) {
                Log.w(TAG, "the pairing was not carried to the PC's new address in time")
                carried(first, moved = false)
            }
        }, CARRY_TIMEOUT_MS)
    }

    // The empty page is there: read the pairing at the old address, or write
    // it at the new one.
    private fun finished(url: String) {
        val step = carry ?: return
        if (!atCarryPage(url, step.page)) return
        val write = step.write
        if (write == null) {
            web.evaluateJavascript(READ_KEYS) { keys -> gotKeys(step, keys) }
        } else {
            web.evaluateJavascript(write) { written -> wrote(step, written == "true") }
        }
    }

    // The keys read: then the pairing they hold, for the new address to prove.
    private fun gotKeys(step: Carrying, keys: String?) {
        if (carry !== step) return
        val write = keys?.let(::writeKeys)
        if (write == null) {
            Log.i(TAG, "no pairing at the PC's old address to carry")
            carried(step, moved = true)
            return
        }
        web.evaluateJavascript(READ_HELD) { read -> gotHeld(step, write, read) }
    }

    // The new address asked, off the main thread, to prove it holds every
    // pairing the carry would write, before anything is written (ADR-417).
    private fun gotHeld(step: Carrying, write: String, read: String?) {
        if (carry !== step) return
        val held = read?.let(::heldOf)
        if (held == null) {
            Log.w(TAG, "the pairing at the PC's old address could not be read to prove")
            carried(step, moved = false)
            return
        }
        probe.run {
            val proven = held.all { Proof.proves(step.link.pc, it) }
            runOnUiThread { proved(step, write, proven) }
        }
    }

    // Proven: the keys written at the new address. Not: nothing carried.
    private fun proved(step: Carrying, write: String, proven: Boolean) {
        if (isDestroyed || carry !== step) return
        val page = carryPage(step.link.pc)
        if (!proven || page == null) {
            Log.w(TAG, "the PC's new address did not prove it holds this tablet's pairing; it is not carried")
            carried(step, moved = false)
            return
        }
        carry = step.copy(page = page, write = write)
        load(page)
    }

    private fun wrote(step: Carrying, written: Boolean) {
        if (carry !== step) return
        if (!written) Log.w(TAG, "the pairing could not be written at the PC's new address")
        carried(step, moved = written)
    }

    // The carry is over, whatever came of it: the new address is kept and
    // opened. One that failed opens at the pairing screen, as without it, and
    // the pairing is still held where it was unless it [moved]: a refused
    // proof costs the next move nothing.
    private fun carried(step: Carrying, moved: Boolean) {
        carry = null
        keep(step.link, step.venue, held = if (moved) step.link.pc else step.from)
    }

    // The app's own way to a page; a stand-in it replaces stops asking, and
    // a carry under way is given up.
    private fun open(url: String) {
        carry = null
        away = null
        probe.stop()
        if (!AppPages.isOwn(url)) {
            opened = url
            forget = true
        }
        load(url)
    }

    // Every page the app loads comes this way: any but the scanning page
    // stops the camera, so no path leaves it running under another page.
    private fun load(url: String) {
        if (url != AppPages.page(AppPages.SCANNING)) stopScan()
        web.loadUrl(url)
    }

    private fun act(action: Action, link: String) {
        when (action) {
            Action.ASK_LOCAL_NETWORK -> {
                if (asking) return
                asking = true
                prefs.edit { putBoolean(ASKED, true) }
                askLocalNetwork.launch(LOCAL_NETWORK)
            }
            Action.OPEN_SETTINGS -> openSettings()
            Action.RETRY -> retry()
            Action.ADDRESS -> typed(typedOf(link).orEmpty())
            Action.PAIR_AGAIN -> pairAgain()
            Action.SCAN -> scan()
            Action.CANCEL_SCAN -> load(AppPages.page(Start.PAIR.page.orEmpty()))
        }
    }

    // "Scan the code": the camera, asked for first; a tablet without one, or
    // a refusal, leaves typing, and the page says so.
    private fun scan() {
        when {
            !packageManager.hasSystemFeature(PackageManager.FEATURE_CAMERA_ANY) ->
                load(AppPages.pairWithoutCamera(AppPages.CAMERA_NONE))
            ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) ==
                PackageManager.PERMISSION_GRANTED -> startScan()
            askingCamera -> Unit
            else -> {
                askingCamera = true
                askCamera.launch(Manifest.permission.CAMERA)
            }
        }
    }

    // The page's words and Cancel, over the camera's picture: the WebView
    // lets it show through while it scans.
    private fun startScan() {
        if (scanner.isRunning || showing != Start.PAIR) return
        web.setBackgroundColor(Color.TRANSPARENT)
        load(AppPages.page(AppPages.SCANNING))
        scanner.start()
    }

    private fun stopScan() {
        if (::scanner.isInitialized) scanner.stop()
        if (::web.isInitialized) web.setBackgroundColor(getColor(R.color.parchment))
    }

    // A code read: checked and asked as if it had been typed.
    private fun scanned(text: String) {
        stopScan()
        typed(text)
    }

    // The address typed on "Pair this tablet": kept, and opened with its code
    // once Android lets the app reach it; or the page again, saying why not.
    private fun typed(text: String) {
        if (candidate != null || carry != null) return // One address asked at a time.
        when (val link = readLink(text)) {
            is Pairing.Link -> {
                candidate = link
                candidateText = text
                showing = null
                show(where())
            }
            is Pairing.Refused -> load(AppPages.pairRefused(link.why, text))
        }
    }

    // "Pair again", from "Can't reach the mez PC": the address is forgotten.
    // The pairing stays in the WebView's storage for that address, and the
    // app keeps where it is held and the venue: the same PC typed again
    // opens as it was, and the same venue's PC at a new address takes the
    // pairing with it (ADR-416).
    private fun pairAgain() {
        prefs.edit { remove(PC) }
        carry = null
        candidate = null
        opened = null
        lastMez = null
        showing = null
        show(where())
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
        load(
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
    private fun answered(mez: Mez) {
        if (isDestroyed) return
        val page = away ?: return
        if (page !== probing) return
        learn(page.url, mez)
        open(page.url)
    }

    // A PC kept before health named its venue is asked once, when its page
    // opens: without the venue, its pairing cannot follow it to a new
    // address (ADR-416).
    private fun askVenue() {
        if (askedVenue || prefs.contains(VENUE)) return
        val health = healthOf(pc) ?: return
        askedVenue = true
        val asked = pc
        probe.once(health) { mez -> if (mez != null) runOnUiThread { learn(asked, mez) } }
    }

    // A PC kept before health named its venue is known by it from its next
    // answer (ADR-416).
    private fun learn(url: String, mez: Mez) {
        val venue = mez.venue ?: return
        if (isDestroyed || pc.isEmpty() || !samePc(url, pc) || prefs.contains(VENUE)) return
        prefs.edit {
            putString(HELD, pc)
            putString(VENUE, venue)
        }
    }

    private fun back() {
        // Back from the camera is Cancel.
        if (scanner.isRunning) {
            load(AppPages.page(Start.PAIR.page.orEmpty()))
            return
        }
        // A carry is a moment, and its pages are not the person's.
        if (carry != null) return
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
        frame.addView(web) // On top: the camera's layer is under it.

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
        val step = carry
        when {
            // A carry goes on in the new WebView from the step it was at:
            // reading and writing are each the same done twice.
            step != null -> load(step.page)
            // The app's own pages are a few kilobytes: they come back as they
            // were, and an address being asked is asked again.
            showing != Start.MEZ || candidate != null -> {
                showing = null
                show(where())
            }
            next == Revival.STOP -> {
                probe.stop()
                probing = null
                standIn(Away(standing?.url ?: lastMez ?: opened ?: pc, AppPages.STOPPED))
            }
            standing != null -> standIn(standing)
            else -> open(lastMez ?: opened ?: pc)
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
        const val PC = "pc"
        const val SEEDED = "seeded"

        // Where the device's pairing is held, and the venue whose mez
        // answered there (ADR-416).
        const val HELD = "held"
        const val VENUE = "venue"

        // A carry is two empty pages, three scripts and one question for the
        // PC, whose connection and read may each take Probe.TIMEOUT_MS.
        const val CARRY_TIMEOUT_MS = 10_000L
    }
}

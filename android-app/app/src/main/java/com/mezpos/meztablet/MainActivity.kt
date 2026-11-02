package com.mezpos.meztablet

import android.Manifest
import android.app.ActivityManager
import android.app.admin.DevicePolicyManager
import android.os.UserManager
import android.content.ActivityNotFoundException
import android.content.ComponentName
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.graphics.Color
import android.os.Build
import android.os.Bundle
import android.os.SystemClock
import android.provider.Settings
import android.util.Log
import android.view.MotionEvent
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
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import java.util.concurrent.RejectedExecutionException

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
        set(value) {
            field = value
            watching = value == Start.MEZ
        }

    // Whether the PC's screens are what the app shows, for the watch, which
    // asks off the main thread.
    @Volatile
    private var watching = false
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
        // Found by the watch, not typed or scanned (Task 5c): a carry not
        // made leaves the tablet where it was.
        val auto: Boolean = false,
        // Read in place, on the PC's page that was showing: a carry not made
        // leaves the screen as it was.
        val here: Boolean = false,
    )

    // Volatile: ShellClient asks which page to serve off the main thread.
    @Volatile
    private var carry: Carrying? = null

    // While the PC's screens show, its address is asked now and then; one
    // that stops answering is looked for by name (ADR-416, Task 5c).
    private val finder = Finder(to = mdnsTo(BuildConfig.MEZ_MDNS))
    private val watch = Ticker(every = WATCH_EVERY_MS, failed = { Log.w(TAG, "the watch for the PC failed", it) }) {
        watchTick()
    }

    // Addresses whose mez did not prove it holds the pairing, and when: not
    // asked again for a while, so a device on the wifi cannot keep a tablet
    // busy, nor its screen flickering.
    private val refused = ConcurrentHashMap<String, Long>()

    // How many ticks in a row the kept address has not answered. The watch's
    // thread alone reads and writes it.
    private var misses = 0

    // The way out's gesture (ADR-419), watched on every touch before the
    // WebView; once made, the rest of that touch is the app's ([claimed]).
    private val hold by lazy { Hold(slop = resources.displayMetrics.density * HOLD_SLOP_DP) }
    private var claimed = false
    private val holdCheck = Runnable { if (hold.made(SystemClock.uptimeMillis())) holdMade() }

    // A way out left alone goes back to mez: a manager who opened it and
    // walked off does not leave Android's settings a tap away.
    private val wayOutIdle = Runnable { if (leaving != null) wayOutBack() }

    /**
     * The way out under way (ADR-419): the PC asked, the pairing it is asked
     * with (none if no PC page was showing), whether it has been asked yet,
     * who may open it as the PC said,
     * whether the PC has said yes and who to, and what Back goes back to: the
     * PC's page that was showing, or the app's page that stood in for it.
     */
    private data class Leaving(
        val pc: String,
        val held: Held? = null,
        val asked: Boolean = false,
        val who: String? = null,
        val open: Boolean = false,
        val by: String = "",
        val chosen: String? = null,
        val back: String? = null,
        val before: Away? = null,
    )

    private var leaving: Leaving? = null

    // The lock (ADR-419, ADR-420): lock task mode as the device owner, screen
    // pinning otherwise. [left]: the way out lifted it, until mez has gone
    // behind another app and come back; [askedToPin]: screen pinning asked
    // for, this start; [inFront]: resumed, the only time a lock is asked.
    private val policy by lazy { getSystemService(DevicePolicyManager::class.java) }
    private val admin by lazy { ComponentName(this, Admin::class.java) }
    private var left = false
    private var askedToPin = false
    private var inFront = false

    // The kiosk's rules as last set, this start: with Android's Wi-Fi panel
    // let run, or not; null before they are set. They are set once, not at
    // every page.
    private var kioskHeld: Boolean? = null

    private val owner: Boolean
        get() = policy?.isDeviceOwnerApp(packageName) == true

    private val inLockTask: Boolean
        get() = getSystemService(ActivityManager::class.java)?.lockTaskModeState
            ?.let { it != ActivityManager.LOCK_TASK_MODE_NONE } == true

    // How the tablet is held, for its pages: a kiosk once it is due to be
    // one, before Android has finished locking it.
    private val locking: Locking?
        get() = if (owner && prefs.getBoolean(LOCKED, false) && !left) Locking.KIOSK else lockingOf(owner, inLockTask)

    // The proof, and the way out's questions, are asked on a thread of their
    // own, not behind the probe's asking of an address that does not answer.
    private val proofs = Executors.newSingleThreadExecutor { task ->
        Thread(task, "mez-proof").apply { isDaemon = true }
    }

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
            noteHeld()
            lockIfDue()
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
        watch.start(after = WATCH_FIRST_MS)
    }

    // On first run, and on coming back from Android's prompt or its Settings,
    // where the local network may have been allowed since.
    override fun onResume() {
        super.onResume()
        inFront = true
        hideSystemBars()
        if (!asking) show(where())
        lockIfDue()
    }

    // A prompt or a dialog over the app can bring the bars back.
    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus) hideSystemBars()
    }

    override fun onPause() {
        inFront = false
        super.onPause()
    }

    override fun onStop() {
        // What the way out lifted holds again when mez is back in front.
        left = false
        probe.stop()
        watch.stop()
        // A way out ends when the app goes behind another.
        if (leaving != null) wayOutBack()
        // A scan ends when the app goes behind another: Scan starts it again.
        if (::scanner.isInitialized && scanner.isRunning) load(AppPages.page(Start.PAIR.page.orEmpty()))
        super.onStop()
    }

    override fun onDestroy() {
        probe.close()
        watch.close()
        proofs.shutdownNow()
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

    // The carry's first step: the pairing read at the address holding it,
    // where finished() reads it on the app's empty page, or in place on the
    // PC's page showing there when the watch found the PC. A carry that has
    // not ended in a few seconds is given up.
    private fun carryFrom(from: String, link: Pairing.Link, venue: String, auto: Boolean = false) {
        val page = carryPage(from)
        if (page == null) {
            if (!auto) keep(link, venue)
            return
        }
        val here = auto && web.url?.let { samePc(it, from) && !isCarryPage(it) } == true
        val first = Carrying(from, link, venue, page, write = null, auto = auto, here = here)
        carry = first
        if (here) readKeys(first) else load(page)
        frame.postDelayed({
            val late = carry
            if (!isDestroyed && late?.link === link) {
                Log.w(TAG, "the pairing was not carried to the PC's new address in time")
                notCarried(late)
            }
        }, CARRY_TIMEOUT_MS)
    }

    // The empty page is there: read the pairing at the old address, or write
    // it at the new one.
    private fun finished(url: String) {
        wayOutAsk(url)
        val step = carry ?: return
        if (!atCarryPage(url, step.page)) return
        val write = step.write
        if (write == null) {
            readKeys(step)
        } else {
            web.evaluateJavascript(write) { written -> wrote(step, written == "true") }
        }
    }

    private fun readKeys(step: Carrying) {
        web.evaluateJavascript(READ_KEYS) { keys -> gotKeys(step, keys) }
    }

    // The keys read: then the pairing they hold, for the new address to prove.
    // With none, a typed address opens as it is; a found one is not followed:
    // an address that has proved nothing does not get the screen.
    private fun gotKeys(step: Carrying, keys: String?) {
        if (carry !== step) return
        val write = keys?.let(::writeKeys)
        if (write == null) {
            Log.i(TAG, "no pairing at the PC's old address to carry")
            if (step.auto) notCarried(step) else carried(step, moved = true)
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
            notCarried(step)
            return
        }
        try {
            proofs.execute {
                val proven = held.all { Proof.proves(step.link.pc, it) }
                runOnUiThread { proved(step, write, proven) }
            }
        } catch (e: RejectedExecutionException) {
            // The activity is gone, and nobody is waiting.
        }
    }

    // Proven: the keys written at the new address. Not: nothing carried.
    private fun proved(step: Carrying, write: String, proven: Boolean) {
        if (isDestroyed || carry !== step) return
        val page = carryPage(step.link.pc)
        if (!proven || page == null) {
            Log.w(TAG, "the PC's new address did not prove it holds this tablet's pairing; it is not carried")
            notCarried(step, refuse = proven.not())
            return
        }
        // The screen leaves the page read in place: a failure from here on
        // brings back "Can't reach the mez PC".
        carry = step.copy(page = page, write = write, here = false)
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

    // A carry not made. Typed or scanned, the new address opens as without
    // one. Found by the watch, the tablet stays where it was: the screen as
    // it was if it was read in place, else "Can't reach the mez PC", asking
    // again only while the app is in front. An address [refuse]d (it did not
    // prove the pairing) is not asked again for a while; a carry that failed
    // on this side is tried again at the next tick.
    private fun notCarried(step: Carrying, refuse: Boolean = false) {
        if (!step.auto) return carried(step, moved = false)
        carry = null
        if (refuse) hostOf(step.link.pc)?.let { refused[it] = SystemClock.elapsedRealtime() }
        if (step.here) return
        standIn(Away(retryOf(away?.url ?: lastMez ?: opened ?: pc, asked = opened), AppPages.UNREACHABLE))
        if (lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)) tryAgain(after = Probe.RETRY_EVERY_MS)
    }

    // The watch's tick, off the main thread: the kept address asked, and if
    // it does not answer as the kept venue's mez, the PC looked for by name.
    // A mez that names no venue is taken as the kept one. Without a kept
    // venue there is nothing to know the PC by, and it is not looked for.
    // Two misses in a row before it looks, so a busy PC or a moment of bad
    // wifi does not move the till; after a minute of misses it looks once a
    // minute; and the kept address is asked once more before the move.
    private fun watchTick() {
        if (!watching || carry != null) return
        val kept = pc
        val venue = prefs.getString(VENUE, null) ?: return
        val health = healthOf(kept) ?: return
        if (isHere(Probe.ask(health), venue)) {
            misses = 0
            return
        }
        misses++
        if (misses < MISSES_BEFORE_LOOKING || (misses > LOOK_EVERY_TICK_UNTIL && misses % LOOK_EVERY_LATER != 0)) return
        val since = SystemClock.elapsedRealtime() - REFUSED_FOR_MS
        refused.entries.removeIf { it.value < since }
        val answered = nearFirst(finder.find().filterNot(refused::containsKey), own = ownAddresses())
        val link = lookFor(kept, venue, answered, ask = Probe::ask) ?: return
        if (isHere(Probe.ask(health), venue)) {
            misses = 0
            return
        }
        runOnUiThread { foundAt(kept, link, venue) }
    }

    // Whether [mez], the kept address's answer, is the kept venue's PC: a
    // mez that names no venue is taken as it.
    private fun isHere(mez: Mez?, venue: String): Boolean = mez != null && (mez.venue == null || mez.venue == venue)

    // The PC answers at another address: its pairing goes there once proved
    // (ADR-417), and the till with it.
    private fun foundAt(kept: String, link: Pairing.Link, venue: String) {
        if (isDestroyed || !watching || pc != kept || carry != null || candidate != null) return
        if (prefs.getString(VENUE, null) != venue || away?.page in setOf(AppPages.STOPPED, AppPages.WAY_OUT)) return
        if (!lifecycle.currentState.isAtLeast(Lifecycle.State.STARTED)) return
        Log.i(TAG, "the PC answers at ${place(link.pc)}: carrying the pairing there")
        carryFrom(held, link, venue, auto = true)
    }

    // The app's own way to a page; a stand-in it replaces stops asking, and
    // a carry or a way out under way is given up.
    private fun open(url: String) {
        carry = null
        leaving = null
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
            // Locked, Android's settings are the way out's (ADR-420).
            Action.OPEN_SETTINGS -> if (locking != null) wayOut() else openSettings()
            Action.RETRY -> retry()
            Action.ADDRESS -> typed(typedOf(link).orEmpty())
            Action.PAIR_AGAIN -> pairAgain()
            Action.SCAN -> scan()
            Action.CANCEL_SCAN -> load(AppPages.page(Start.PAIR.page.orEmpty()))
            Action.WAY_OUT_OPEN -> wayOutOpen(link)
            Action.WAY_OUT_BACK -> wayOutBack()
            Action.WAY_OUT_SETTINGS -> if (leaving?.open == true) wayOutSettings()
            Action.WAY_OUT_PAIR_AGAIN -> if (leaving?.open == true) pairAgain()
            Action.WAY_OUT_LEAVE -> if (leaving?.open == true) wayOutLeave()
            Action.WAY_OUT_RELEASE -> if (leaving?.open == true) wayOutRelease()
            Action.WIFI -> if (away?.page == AppPages.UNREACHABLE) wifiSettings()
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
        leaving = null
        away = null
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
        // Another stand-in in the way out's place ends it (a renderer lost,
        // "The screen stopped"): the next hold starts one afresh.
        if (page.page != AppPages.WAY_OUT) leaving = null
        val way = leaving
        load(
            when {
                page.page == AppPages.UNREACHABLE -> AppPages.unreachable(page.url, locking)
                // Lost before the PC was asked: asked once this page loads.
                page.page == AppPages.WAY_OUT && way != null && way.held != null && !way.asked -> wayOutAsking()
                page.page == AppPages.WAY_OUT && way != null -> wayOutPage(way, why = null)
                else -> AppPages.page(page.page)
            },
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

    // The PC's page is at an address other than where the pairing was held
    // (a carry not made, then the tablet paired there by hand): once the
    // page's address holds a pairing, that is the one in use, and the next
    // move carries it.
    private fun noteHeld() {
        val here = pc
        if (here.isEmpty() || held == here || carry != null) return
        web.evaluateJavascript(HAS_PAIRING) { has ->
            if (has == "true" && pc == here && carry == null) prefs.edit { putString(HELD, here) }
        }
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

    // Every touch, before the WebView: the way out's gesture is watched for,
    // and the touch passed on, until the hold is made.
    override fun dispatchTouchEvent(event: MotionEvent): Boolean {
        val action = event.actionMasked
        // The rest of the touch the hold was made by is the app's; a new
        // touch is the page's again, whatever became of the last one's end.
        if (claimed && action != MotionEvent.ACTION_DOWN) {
            if (action == MotionEvent.ACTION_UP || action == MotionEvent.ACTION_CANCEL) claimed = false
            return true
        }
        claimed = false
        val fingers = when (action) {
            MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> 0
            MotionEvent.ACTION_POINTER_UP -> event.pointerCount - 1
            else -> event.pointerCount
        }
        val places = FloatArray(event.pointerCount * 2) { if (it % 2 == 0) event.getX(it / 2) else event.getY(it / 2) }
        val due = hold.touch(fingers, places, event.eventTime)
        frame.removeCallbacks(holdCheck)
        if (due != null) frame.postDelayed(holdCheck, maxOf(0L, due - SystemClock.uptimeMillis()))
        return super.dispatchTouchEvent(event)
    }

    // The hold is made: the page's touch is cancelled, so nothing on the till
    // is pressed, and the way out opens.
    private fun holdMade() {
        hold.reset()
        claimed = true
        val now = SystemClock.uptimeMillis()
        val cancel = MotionEvent.obtain(now, now, MotionEvent.ACTION_CANCEL, 0f, 0f, 0)
        super.dispatchTouchEvent(cancel)
        cancel.recycle()
        wayOut()
    }

    // The way out (ADR-419). From the PC's page, the pairing is read there and
    // the PC asked who may open it; from any other, there is no PC to check a
    // PIN, and the page says so.
    private fun wayOut() {
        if (away?.page == AppPages.WAY_OUT || carry != null || candidate != null || scanner.isRunning) return
        val url = web.url.orEmpty()
        val here = pc
        val onPc = here.isNotEmpty() && samePc(url, here) && !isCarryPage(url)
        val step = Leaving(pc = here, back = url.takeIf { onPc }, before = away)
        leaving = step
        probe.stop()
        away = Away(step.back ?: step.before?.url ?: lastMez ?: here, AppPages.WAY_OUT)
        stillThere()
        if (!onPc) {
            load(wayOutPage(step, why = if (here.isEmpty()) Opened.NOT_PAIRED.why else Opened.NO_ANSWER.why))
            return
        }
        web.evaluateJavascript(READ_HELD) { read -> wayOutHeld(step, read) }
    }

    private fun wayOutHeld(step: Leaving, read: String?) {
        if (leaving !== step) return
        val held = read?.let(::heldOf)?.firstOrNull()
        if (held == null) {
            load(wayOutPage(step, why = Opened.NOT_PAIRED.why))
            return
        }
        leaving = step.copy(held = held)
        // The till goes at once: the person sees their hold was made. The PC
        // is asked once this page has loaded (wayOutAsk).
        load(wayOutAsking())
    }

    private fun wayOutAsking() = AppPages.wayOut(who = null, why = WAY_OUT_ASKING)

    // "Asking the mez PC" has loaded: now the PC is asked who may open the
    // way out. Asked any sooner, a quick answer's page was overtaken by this
    // one, still loading, and the way out stayed on "Asking".
    private fun wayOutAsk(url: String) {
        val step = leaving ?: return
        val held = step.held ?: return
        if (step.asked || url.substringBefore('#') != wayOutAsking().substringBefore('#')) return
        val next = step.copy(asked = true)
        leaving = next
        ask { WayOutAsk.who(next.pc, held.token).let { who -> runOnUiThread { wayOutWho(next, who) } } }
    }

    private fun wayOutWho(step: Leaving, who: String?) {
        if (isDestroyed || leaving !== step) return
        val next = step.copy(who = who)
        leaving = next
        load(wayOutPage(next, why = if (who == null) Opened.NO_ANSWER.why else null))
    }

    // A PIN typed on the way out's page, for the PC to check.
    private fun wayOutOpen(link: String) {
        val step = leaving ?: return
        val held = step.held ?: return
        if (step.open) return
        val staff = queryOf(link, "staff").orEmpty()
        val pin = queryOf(link, "pin").orEmpty()
        val asking = step.copy(chosen = staff)
        leaving = asking
        stillThere()
        ask {
            val (opened, by) = WayOutAsk.open(asking.pc, held, staff, pin)
            runOnUiThread { wayOutOpened(asking, opened, by) }
        }
    }

    private fun wayOutOpened(step: Leaving, opened: Opened, by: String?) {
        if (isDestroyed || leaving !== step) return
        if (opened != Opened.OPEN) {
            load(wayOutPage(step, why = opened.why))
            return
        }
        Log.i(TAG, "the way out was opened")
        val next = step.copy(open = true, by = by.orEmpty())
        leaving = next
        stillThere()
        load(wayOutPage(next, why = null))
    }

    private fun wayOutPage(step: Leaving, why: String?): String =
        if (step.open) AppPages.wayOutOpen(step.by, locking) else AppPages.wayOut(step.who, why, step.chosen)

    // Back to what the way out was opened over: the PC's page, or the app's
    // page that stood in for it, or where the app starts.
    private fun wayOutBack() {
        val step = leaving ?: return
        leaving = null
        frame.removeCallbacks(wayOutIdle)
        val before = step.before
        val back = step.back
        when {
            back != null -> open(back)
            before != null -> {
                standIn(before)
                if (before.page == AppPages.UNREACHABLE) tryAgain(after = 0)
            }
            else -> {
                away = null
                showing = null
                show(where())
            }
        }
    }

    // Android's settings, the way out open: mez is back behind them, as it
    // was, for when the person comes back.
    private fun wayOutSettings() {
        left = true
        wayOutBack()
        // The lock lifted while they are open; it holds again when mez is
        // back in front (lockIfDue).
        stopLock()
        try {
            startActivity(Intent(Settings.ACTION_SETTINGS))
        } catch (e: ActivityNotFoundException) {
            Log.w(TAG, "no Settings to open from the way out", e)
            relock()
        }
    }

    // Nothing opened in mez's place: the lock holds again at once.
    private fun relock() {
        left = false
        lockIfDue()
    }

    // Locks the tablet to mez when it is due (ADR-420): the till has shown
    // once on this tablet, and from then on on every page, while mez is in
    // front. As the device owner, the kiosk's rules are set each time.
    // Called from onResume and when the PC's page shows; Android refuses a
    // lock asked from behind another app, which is let go, and asked again
    // at the next resume.
    private fun lockIfDue() {
        if (!inFront || isFinishing || isDestroyed) return
        val kiosk = owner
        // Made the device owner again after a release: a kiosk again.
        if (kiosk && prefs.getBoolean(RELEASED, false)) prefs.edit { remove(RELEASED) }
        val released = prefs.getBoolean(RELEASED, false)
        if (showing == Start.MEZ && pc.isNotEmpty() && !released && !prefs.getBoolean(LOCKED, false)) {
            prefs.edit { putBoolean(LOCKED, true) }
        }
        val engaged = prefs.getBoolean(LOCKED, false)
        if (kiosk && engaged) holdAsKiosk()
        if (!shouldLock(kiosk, engaged, inLockTask, left, askedToPin)) return
        try {
            startLockTask()
            if (!kiosk) askedToPin = true
        } catch (e: IllegalStateException) {
            Log.w(TAG, "the tablet could not be locked to mez", e)
        } catch (e: IllegalArgumentException) {
            Log.w(TAG, "the tablet could not be locked to mez", e)
        } catch (e: SecurityException) {
            Log.w(TAG, "the tablet could not be locked to mez", e)
        }
    }

    // A kiosk's rules, as the device owner, set once a start: mez alone runs
    // while locked (and Android's Wi-Fi panel while it is open from "Can't
    // reach the mez PC"), the power menu kept, no lock screen, mez the
    // tablet's home; no factory reset, safe mode or new user from Settings,
    // which the Wi-Fi panel can reach; and the local network allowed, so
    // that a kiosk is never stuck on a permission only Settings could give.
    private fun holdAsKiosk(wifi: Boolean = false) {
        if (kioskHeld == wifi) return
        val rules = policy ?: return
        try {
            val settings = if (wifi) wifiPackage() else null
            rules.setLockTaskPackages(admin, listOfNotNull(packageName, settings).toTypedArray())
            if (kioskHeld == null) {
                rules.setLockTaskFeatures(admin, DevicePolicyManager.LOCK_TASK_FEATURE_GLOBAL_ACTIONS)
                rules.setKeyguardDisabled(admin, true)
                val home = IntentFilter(Intent.ACTION_MAIN).apply {
                    addCategory(Intent.CATEGORY_HOME)
                    addCategory(Intent.CATEGORY_DEFAULT)
                }
                rules.addPersistentPreferredActivity(admin, home, ComponentName(this, MainActivity::class.java))
                for (rule in KIOSK_RESTRICTIONS) rules.addUserRestriction(admin, rule)
                // False where Android has no such permission (before 17).
                rules.setPermissionGrantState(admin, packageName, LOCAL_NETWORK, DevicePolicyManager.PERMISSION_GRANT_STATE_GRANTED)
            }
            kioskHeld = wifi
        } catch (e: SecurityException) {
            Log.w(TAG, "the kiosk's rules could not be set", e)
        } catch (e: IllegalArgumentException) {
            Log.w(TAG, "the kiosk's rules could not be set", e)
        }
    }

    private fun stopLock() {
        if (!inLockTask) return
        try {
            stopLockTask()
        } catch (e: IllegalStateException) {
            Log.w(TAG, "the lock could not be lifted", e)
        }
    }

    // "Wi-Fi settings" on a kiosk whose PC does not answer: Android's Wi-Fi
    // panel, let run inside the lock until mez is back in front, with no PIN,
    // since the PIN needs the PC (ADR-420).
    private fun wifiSettings() {
        if (locking != Locking.KIOSK) return
        holdAsKiosk(wifi = true)
        try {
            startActivity(Intent(Settings.Panel.ACTION_WIFI))
        } catch (e: ActivityNotFoundException) {
            holdAsKiosk()
            Log.w(TAG, "no Wi-Fi settings to open", e)
        }
    }

    private fun wifiPackage(): String? =
        packageManager.queryIntentActivities(Intent(Settings.Panel.ACTION_WIFI), 0).firstOrNull()?.activityInfo?.packageName

    // "Leave mez": the lock lifted for a visit to the tablet's own home app.
    // mez stays the home a kiosk is held to, so Home, or a reboot, brings the
    // till back, locked again (ADR-420). With no home app of the tablet's
    // own, Android's choice of one.
    private fun wayOutLeave() {
        left = true
        stopLock()
        wayOutBack()
        val other = otherHome(homeApps(), packageName)
        if (other == null) {
            if (!openHomeChoice()) relock()
            return
        }
        try {
            startActivity(homeIntent().setClassName(other.pkg, other.activity))
        } catch (e: ActivityNotFoundException) {
            Log.w(TAG, "the tablet's home app could not be opened", e)
            if (!openHomeChoice()) relock()
        }
    }

    private fun homeIntent() = Intent(Intent.ACTION_MAIN).addCategory(Intent.CATEGORY_HOME)

    private fun homeApps(): List<HomeApp> =
        packageManager.queryIntentActivities(homeIntent(), 0).map {
            HomeApp(it.activityInfo.packageName, it.activityInfo.name, it.priority)
        }

    // "Release this tablet": an ordinary tablet again, mez no longer its
    // device owner nor the home it is held to, and Android's choice of home
    // app; locking it again takes a computer (adb).
    private fun wayOutRelease() {
        if (!owner) return
        release()
        // Set again in full at the next lock, whatever release undid.
        kioskHeld = null
        if (owner) {
            // Not released: the way out stays open, and says nothing new.
            Log.w(TAG, "the tablet could not be released; it is still locked to mez")
            lockIfDue()
            return
        }
        left = true
        // Not asked to pin either: an ordinary tablet, until it is made the
        // device owner again.
        prefs.edit {
            remove(LOCKED)
            putBoolean(RELEASED, true)
        }
        wayOutBack()
        if (!openHomeChoice()) relock()
    }

    // Each rule undone apart, so that one refused does not keep the rest.
    private fun release() {
        stopLock()
        val rules = policy ?: return
        val steps = listOf<() -> Unit>(
            { rules.clearPackagePersistentPreferredActivities(admin, packageName) },
            { rules.setKeyguardDisabled(admin, false) },
            { rules.setLockTaskPackages(admin, emptyArray()) },
            { for (rule in KIOSK_RESTRICTIONS) rules.clearUserRestriction(admin, rule) },
            {
                rules.setPermissionGrantState(admin, packageName, LOCAL_NETWORK, DevicePolicyManager.PERMISSION_GRANT_STATE_DEFAULT)
            },
            {
                @Suppress("DEPRECATION")
                rules.clearDeviceOwnerApp(packageName)
            },
        )
        for (step in steps) {
            try {
                step()
            } catch (e: SecurityException) {
                Log.w(TAG, "a rule of the kiosk could not be undone", e)
            } catch (e: IllegalArgumentException) {
                Log.w(TAG, "a rule of the kiosk could not be undone", e)
            }
        }
    }

    private fun openHomeChoice(): Boolean = try {
        startActivity(Intent(Settings.ACTION_HOME_SETTINGS))
        true
    } catch (e: ActivityNotFoundException) {
        Log.w(TAG, "no home app choice to open", e)
        false
    }

    // The way out is in use: it goes back to mez a minute after the last of it.
    private fun stillThere() {
        frame.removeCallbacks(wayOutIdle)
        frame.postDelayed(wayOutIdle, WAY_OUT_IDLE_MS)
    }

    // A question for the PC, on the proofs' thread.
    private fun ask(task: () -> Unit) {
        try {
            proofs.execute(task)
        } catch (e: RejectedExecutionException) {
            // The activity is gone, and nobody is waiting.
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
            // reading and writing are each the same done twice. A read in
            // place goes on at the app's empty page: the page it read on is
            // gone with the renderer.
            step != null -> {
                carry = step.copy(here = false)
                load(step.page)
            }
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

        // Whether the tablet has been locked to mez: the till has shown on it
        // since it was last released (ADR-420).
        const val LOCKED = "locked"

        // What a kiosk forbids from Settings, which Android's Wi-Fi panel
        // reaches (ADR-420): a factory reset, safe mode (from the power menu
        // the kiosk keeps, it starts the tablet without mez), a new user.
        val KIOSK_RESTRICTIONS = listOf(
            UserManager.DISALLOW_FACTORY_RESET,
            UserManager.DISALLOW_SAFE_BOOT,
            UserManager.DISALLOW_ADD_USER,
        )

        // Released by the way out: no lock asked until the app is made the
        // device owner again.
        const val RELEASED = "released"

        // Where the device's pairing is held, and the venue whose mez
        // answered there (ADR-416).
        const val HELD = "held"
        const val VENUE = "venue"

        // A carry is two empty pages, three scripts and a question for the
        // PC per pairing held, whose connection and read may each take
        // Probe.TIMEOUT_MS.
        const val CARRY_TIMEOUT_MS = 20_000L

        // How often the watch asks the kept address while the PC's screens
        // show, and how soon after the app comes to the front.
        const val WATCH_EVERY_MS = 15_000L
        const val WATCH_FIRST_MS = 3_000L

        // How far a finger may wander while the way out's two are held (ADR-419).
        const val HOLD_SLOP_DP = 24f

        // How long the way out waits on the person before it goes back to mez.
        const val WAY_OUT_IDLE_MS = 60_000L

        // The way out's page while the PC is asked who may open it.
        const val WAY_OUT_ASKING = "asking"

        // How long an address that proved nothing is left alone.
        const val REFUSED_FOR_MS = 120_000L

        // Ticks the kept address must miss in a row before the PC is looked
        // for; then it is looked for at each tick up to the fifth miss (a
        // minute), and after that at every fourth (once a minute).
        const val MISSES_BEFORE_LOOKING = 2
        const val LOOK_EVERY_TICK_UNTIL = 5
        const val LOOK_EVERY_LATER = 4
    }
}

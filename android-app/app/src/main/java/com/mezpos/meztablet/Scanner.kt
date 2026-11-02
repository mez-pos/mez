package com.mezpos.meztablet

import android.graphics.Bitmap
import android.graphics.Matrix
import android.graphics.SurfaceTexture
import android.hardware.display.DisplayManager
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Log
import android.view.Surface
import android.view.TextureView
import android.view.View
import android.widget.FrameLayout
import androidx.activity.ComponentActivity
import androidx.camera.core.CameraInfoUnavailableException
import androidx.camera.core.CameraSelector
import androidx.camera.core.CameraState
import androidx.camera.core.Preview
import androidx.camera.core.SurfaceRequest
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.core.content.ContextCompat
import androidx.core.graphics.createBitmap
import androidx.lifecycle.LiveData
import java.util.concurrent.ExecutionException
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.RejectedExecutionException
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import kotlin.math.max

/**
 * The camera, for "Pair this tablet" (ADR-415). CameraX's preview is drawn on
 * a TextureView in [holder], under the WebView, whose page draws the words
 * and Cancel over it. A few times a second a small copy of the picture is
 * read by ZXing ([qrText]) on a thread of its own: one camera stream, the one
 * every camera app uses, rather than a second for analysis, which a cheap
 * tablet's camera may not keep up (an emulator's did not). The first code
 * read is told to [found], once, and the camera stops; [failed] if the
 * camera cannot be opened or shows nothing. Each [start] is a session of its
 * own: nothing from an earlier one is told in a later one.
 */
class Scanner(
    private val activity: ComponentActivity,
    private val holder: FrameLayout,
    private val found: (String) -> Unit,
    private val failed: () -> Unit,
) {
    private val main = ContextCompat.getMainExecutor(activity)
    private val handler = Handler(Looper.getMainLooper())
    private val displays = activity.getSystemService(DisplayManager::class.java)

    // Main thread only, for the session under way.
    private var running = false
    private var session = 0
    private var provider: ProcessCameraProvider? = null
    private var preview: Preview? = null
    private var state: LiveData<CameraState>? = null
    private var texture: TextureView? = null
    private var reader: ExecutorService? = null
    private var copy: Bitmap? = null
    private var pixels: IntArray? = null

    // When the camera last drew a picture (SystemClock.elapsedRealtime): one
    // that stops drawing, or never starts, is given up on.
    private var lastPicture = 0L

    // The camera's sensor, a turn from the tablet's natural way up; and how
    // its picture lies in the texture ([fit]), once CameraX has said.
    private var sensor = 0
    private var lie: Fit? = null

    // The session whose reader may still tell a code (0: none), and one copy
    // read at a time.
    private val telling = AtomicInteger()
    private val reading = AtomicBoolean()

    private val look = object : Runnable {
        override fun run() {
            if (!running) return
            if (SystemClock.elapsedRealtime() - lastPicture > STILL_MS) {
                fail("no picture from the camera for ${STILL_MS / MS_PER_S} s", null)
                return
            }
            readWhatIsShown()
            handler.postDelayed(this, LOOK_EVERY_MS)
        }
    }

    // A tablet turned while it scans: the picture is turned with it.
    private val turned = object : DisplayManager.DisplayListener {
        override fun onDisplayChanged(displayId: Int) {
            preview?.targetRotation = rotation()
        }
        override fun onDisplayAdded(displayId: Int) = Unit
        override fun onDisplayRemoved(displayId: Int) = Unit
    }

    val isRunning: Boolean get() = running

    fun start() {
        if (running) return
        running = true
        val mine = ++session
        telling.set(mine)
        holder.visibility = View.VISIBLE
        val future = ProcessCameraProvider.getInstance(activity)
        future.addListener({
            if (!running || session != mine) return@addListener
            try {
                bind(future.get(), mine)
            } catch (e: ExecutionException) {
                fail("CameraX did not start", e)
            } catch (e: CameraInfoUnavailableException) {
                fail("no camera to read", e)
            } catch (e: IllegalArgumentException) {
                fail("the camera would not bind", e)
            } catch (e: IllegalStateException) {
                fail("the camera would not bind", e)
            }
        }, main)
    }

    fun stop() {
        if (!running) return
        running = false
        session++
        telling.set(0)
        handler.removeCallbacksAndMessages(null)
        displays.unregisterDisplayListener(turned)
        state?.removeObservers(activity)
        state = null
        provider?.unbindAll()
        reader?.shutdown()
        reader = null
        preview = null
        texture = null
        lie = null
        holder.removeAllViews()
        holder.visibility = View.GONE
        copy?.recycle()
        copy = null
        pixels = null
    }

    private fun bind(camera: ProcessCameraProvider, mine: Int) {
        provider = camera
        val selector = when {
            camera.hasCamera(CameraSelector.DEFAULT_BACK_CAMERA) -> CameraSelector.DEFAULT_BACK_CAMERA
            // A tablet on a stand may have only the camera that faces the room.
            camera.hasCamera(CameraSelector.DEFAULT_FRONT_CAMERA) -> CameraSelector.DEFAULT_FRONT_CAMERA
            else -> {
                fail("no camera on this tablet", null)
                return
            }
        }
        val view = TextureView(activity)
        texture = view
        holder.addView(view, FrameLayout.LayoutParams(FULL, FULL))
        val shown = Preview.Builder().setTargetRotation(rotation()).build()
        preview = shown
        shown.setSurfaceProvider(main) { request -> show(request, view, mine) }
        reader = Executors.newSingleThreadExecutor()

        camera.unbindAll()
        val bound = camera.bindToLifecycle(activity, selector, shown)
        sensor = bound.cameraInfo.sensorRotationDegrees
        displays.registerDisplayListener(turned, handler)
        // A camera in use, disabled or broken says so here; one that draws
        // nothing, or stops drawing, is given up on by [look].
        state = bound.cameraInfo.cameraState.also { live ->
            live.observe(activity) { now ->
                if (now.error?.type == CameraState.ErrorType.CRITICAL && session == mine) {
                    fail("the camera reported ${now.error?.code}", null)
                }
            }
        }
        lastPicture = SystemClock.elapsedRealtime()
        handler.postDelayed(look, LOOK_EVERY_MS)
    }

    // The main thread copies the camera's picture, small, at the shape the
    // texture draws it: getBitmap leaves out the view's transform, so a copy
    // at the screen's shape squashed the code past reading. Not turned, which
    // a QR code does not mind. The reader's thread reads it; the first code
    // read goes back to the main thread, which stops the camera first.
    private fun readWhatIsShown() {
        val view = texture ?: return
        val pool = reader ?: return
        val drawn = lie ?: return
        if (!view.isAvailable || drawn.drawnW == 0 || drawn.drawnH == 0) return
        if (!reading.compareAndSet(false, true)) return
        var handedOver = false
        try {
            val longest = max(drawn.drawnW, drawn.drawnH).toLong()
            val wide = (LOOK_LONGEST * drawn.drawnW / longest).toInt().coerceAtLeast(1)
            val tall = (LOOK_LONGEST * drawn.drawnH / longest).toInt().coerceAtLeast(1)
            val bitmap = copy?.takeIf { it.width == wide && it.height == tall }
                ?: createBitmap(wide, tall).also { copy?.recycle(); copy = it }
            view.getBitmap(bitmap)
            // One array, reused: the reader has it only while [reading] is set.
            val colours = pixels?.takeIf { it.size == wide * tall } ?: IntArray(wide * tall).also { pixels = it }
            bitmap.getPixels(colours, 0, wide, 0, 0, wide, tall)
            val mine = session
            pool.execute { read(colours, wide, tall, mine) }
            handedOver = true
        } catch (e: RejectedExecutionException) {
            // Stopped between a copy and its reading.
        } finally {
            if (!handedOver) reading.set(false)
        }
    }

    // The reader's thread. A frame ZXing trips on is skipped, never the end of
    // the app; and [reading] is let go whatever happens.
    private fun read(colours: IntArray, wide: Int, tall: Int, mine: Int) {
        try {
            val text = qrText(luminance(colours), wide, wide, tall) ?: return
            if (!telling.compareAndSet(mine, 0)) return
            main.execute {
                if (!running || session != mine) return@execute
                stop()
                found(text)
            }
        } catch (e: RuntimeException) {
            Log.w(TAG, "scanning: a frame could not be read", e)
        } finally {
            reading.set(false)
        }
    }

    // The camera draws into a TextureView, composed with the views above it,
    // so the WebView's page shows it through; a SurfaceView's picture sits
    // under the window, and a WebView over it showed the window instead.
    private fun show(request: SurfaceRequest, view: TextureView, mine: Int) {
        val size = request.resolution
        var given: Surface? = null
        var cameraDone = false
        var viewGone = false
        fun provide(target: SurfaceTexture) {
            if (!running || session != mine || given != null) return
            target.setDefaultBufferSize(size.width, size.height)
            val surface = Surface(target)
            given = surface
            request.provideSurface(surface, main) {
                surface.release()
                cameraDone = true
                // The view let the texture go while the camera still drew.
                if (viewGone) target.release()
            }
        }
        request.setTransformationInfoListener(main) { info ->
            val applied = if (info.hasCameraTransform()) sensor else 0
            val now = fit(size.width, size.height, info.rotationDegrees, applied)
            lie = now
            upright(view, now)
        }
        view.surfaceTextureListener = object : TextureView.SurfaceTextureListener {
            override fun onSurfaceTextureAvailable(target: SurfaceTexture, width: Int, height: Int) = provide(target)
            override fun onSurfaceTextureSizeChanged(target: SurfaceTexture, width: Int, height: Int) {
                lie?.let { upright(view, it) }
            }
            // Released here only once the camera has stopped drawing into it.
            override fun onSurfaceTextureDestroyed(target: SurfaceTexture): Boolean {
                viewGone = true
                return given == null || cameraDone
            }
            override fun onSurfaceTextureUpdated(target: SurfaceTexture) {
                lastPicture = SystemClock.elapsedRealtime()
            }
        }
        val ready = view.surfaceTexture
        if (view.isAvailable && ready != null) provide(ready)
    }

    // A TextureView stretches the picture, as its texture draws it, over the
    // whole view. Undo the stretch, make the turn still owed ([fit]), and
    // scale it to fill the view, centred, without squashing it.
    private fun upright(view: TextureView, lie: Fit) {
        val viewW = view.width.toFloat()
        val viewH = view.height.toFloat()
        if (viewW == 0f || viewH == 0f || lie.drawnW == 0 || lie.drawnH == 0) return
        val fill = max(viewW / lie.shownW, viewH / lie.shownH)
        val cx = viewW / 2
        val cy = viewH / 2
        view.setTransform(
            Matrix().apply {
                postScale(lie.drawnW / viewW, lie.drawnH / viewH, cx, cy)
                postRotate(lie.turn.toFloat(), cx, cy)
                postScale(fill, fill, cx, cy)
            },
        )
    }

    private fun rotation(): Int = ContextCompat.getDisplayOrDefault(activity).rotation

    private fun fail(why: String, e: Exception?) {
        Log.w(TAG, "scanning: $why", e)
        stop()
        failed()
    }

    private companion object {
        const val TAG = "mez"
        const val FULL = FrameLayout.LayoutParams.MATCH_PARENT

        // Long enough to read a code that fills the frame on the page, small
        // enough to copy and read several times a second.
        const val LOOK_LONGEST = 960L
        const val LOOK_EVERY_MS = 200L

        // A camera that has drawn nothing for this long is not going to.
        const val STILL_MS = 8_000L
        const val MS_PER_S = 1_000L
    }
}

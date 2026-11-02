package com.mezpos.meztablet

import java.io.IOException
import java.net.HttpURLConnection
import java.net.URI
import java.net.URISyntaxException
import java.util.concurrent.Executors
import java.util.concurrent.RejectedExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

/**
 * Asks the PC whether mez answers, on a thread of its own, every [every] ms
 * until it does; then calls [answered], on that thread, once. Asking the
 * health check rather than loading the page again keeps WebView's own error
 * page off the screen between tries.
 */
class Probe(
    private val every: Long = RETRY_EVERY_MS,
    private val answered: () -> Unit,
) {
    private val thread = Executors.newSingleThreadScheduledExecutor { task ->
        Thread(task, "mez-probe").apply { isDaemon = true }
    }

    // Each start is a new generation; a try from an older one does nothing.
    private val generation = AtomicInteger()

    /** Asks [health] after [after] ms, then every [every] ms. Replaces asking already under way. */
    fun start(health: String, after: Long) {
        ask(health, after, generation.incrementAndGet())
    }

    /** Stops asking. A try already on the wire finishes, and is not told. */
    fun stop() {
        generation.incrementAndGet()
    }

    /** Stops for good: the activity is gone. */
    fun close() {
        stop()
        thread.shutdownNow()
    }

    private fun ask(health: String, after: Long, mine: Int) {
        try {
            thread.schedule({
                if (generation.get() != mine) return@schedule
                when {
                    !answers(health) -> ask(health, every, mine)
                    // Only the generation still asking is told, and once.
                    generation.compareAndSet(mine, mine + 1) -> answered()
                }
            }, after, TimeUnit.MILLISECONDS)
        } catch (e: RejectedExecutionException) {
            // Closed, between a try and the next.
        }
    }

    companion object {
        /** How often "Can't reach the mez PC" tries again by itself. */
        const val RETRY_EVERY_MS = 5_000L

        /** A PC on the venue's wifi answers in milliseconds; one that has not in this long is not there. */
        const val TIMEOUT_MS = 3_000

        /** The start of mez's health answer (`internal/api/health.go`), whatever else it says. */
        const val MEZ_HEALTH = "{\"status\":\"ok\""

        // Enough of the answer to find [MEZ_HEALTH] at its start.
        private const val READ_CHARS = 256

        /**
         * Whether [health], the PC's health check, answers as mez does: a 200
         * that starts with [MEZ_HEALTH]. Another device given the PC's old
         * address, answering 200 to everything, is not mez.
         */
        fun answers(health: String): Boolean = try {
            val call = URI(health).toURL().openConnection() as HttpURLConnection
            call.connectTimeout = TIMEOUT_MS
            call.readTimeout = TIMEOUT_MS
            call.useCaches = false
            call.instanceFollowRedirects = false
            try {
                call.responseCode == HttpURLConnection.HTTP_OK && call.inputStream.bufferedReader().use { body ->
                    val start = CharArray(READ_CHARS)
                    val read = body.read(start)
                    read > 0 && String(start, 0, read).startsWith(MEZ_HEALTH)
                }
            } finally {
                call.disconnect()
            }
        } catch (e: IOException) {
            false
        } catch (e: URISyntaxException) {
            false
        } catch (e: IllegalArgumentException) {
            false
        }
    }
}

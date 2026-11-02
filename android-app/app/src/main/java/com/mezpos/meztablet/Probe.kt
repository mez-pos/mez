package com.mezpos.meztablet

import java.io.IOException
import java.io.Reader
import java.net.HttpURLConnection
import java.net.URI
import java.net.URISyntaxException
import java.util.concurrent.Executors
import java.util.concurrent.RejectedExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

/** mez's answer to its health check: the venue it serves, if it said which (ADR-416). */
data class Mez(val venue: String?)

/** What the PC answered a GET: its status, and the start of the body of a 200. */
class Fetched(val status: Int, val body: String)

/**
 * Asks the PC whether mez answers, on a thread of its own, every [every] ms
 * until it does; then calls [answered], on that thread, once, with what it
 * said. Asking the health check rather than loading the page again keeps
 * WebView's own error page off the screen between tries.
 */
class Probe(
    private val every: Long = RETRY_EVERY_MS,
    private val answered: (Mez) -> Unit,
) {
    private val thread = Executors.newSingleThreadScheduledExecutor { task ->
        Thread(task, "mez-probe").apply { isDaemon = true }
    }

    // Each start is a new generation; a try from an older one does nothing.
    private val generation = AtomicInteger()

    /** Asks [health] after [after] ms, then every [every] ms. Replaces asking already under way. */
    fun start(health: String, after: Long) {
        tryAfter(health, after, generation.incrementAndGet())
    }

    /** Stops asking. A try already on the wire finishes, and is not told. */
    fun stop() {
        generation.incrementAndGet()
    }

    /**
     * Asks [health] once, now, and tells [then] what mez said, or null if it
     * did not answer, on the probe's thread: an address is kept only once mez
     * has (ADR-414), with the venue it named (ADR-416).
     */
    fun once(health: String, then: (Mez?) -> Unit) {
        try {
            thread.execute { then(ask(health)) }
        } catch (e: RejectedExecutionException) {
            // Closed: the activity is gone, and nobody is waiting.
        }
    }

    /** Runs [task] on the probe's thread: a question for a PC that is not its health. */
    fun run(task: () -> Unit) {
        try {
            thread.execute(task)
        } catch (e: RejectedExecutionException) {
            // Closed: the activity is gone, and nobody is waiting.
        }
    }

    /** Stops for good: the activity is gone. */
    fun close() {
        stop()
        thread.shutdownNow()
    }

    private fun tryAfter(health: String, after: Long, mine: Int) {
        try {
            thread.schedule({
                if (generation.get() != mine) return@schedule
                val mez = ask(health)
                when {
                    mez == null -> tryAfter(health, every, mine)
                    // Only the generation still asking is told, and once.
                    generation.compareAndSet(mine, mine + 1) -> answered(mez)
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

        // Enough of the answer for all it says: a few hundred characters.
        private const val READ_CHARS = 1024

        // A venue's public id (`internal/domain/id`), as health writes it.
        private val VENUE = Regex("\"venue\":\"([A-Za-z0-9_]{1,64})\"")

        /**
         * What [health], the PC's health check, says: mez's answer, or null if
         * something else answered or nothing did. Another device given the
         * PC's old address, answering 200 to everything, is not mez.
         */
        fun ask(health: String): Mez? = fetch(health)?.let { heard(it.status, it.body) }

        /**
         * A GET of [url] on the PC, or a POST of [post] as JSON, on the
         * caller's thread, with the device's [bearer] token if given: its
         * status, and the start of its body, an error's included. Null if
         * nothing answered. No redirect is followed, nothing is cached, and
         * nothing waits longer than [TIMEOUT_MS] at a time.
         */
        fun fetch(url: String, bearer: String? = null, post: String? = null): Fetched? = try {
            val call = URI(url).toURL().openConnection() as HttpURLConnection
            call.connectTimeout = TIMEOUT_MS
            call.readTimeout = TIMEOUT_MS
            call.useCaches = false
            call.instanceFollowRedirects = false
            if (bearer != null) call.setRequestProperty("Authorization", "Bearer $bearer")
            if (post != null) {
                call.requestMethod = "POST"
                call.doOutput = true
                call.setRequestProperty("Content-Type", "application/json")
                call.outputStream.use { it.write(post.toByteArray(Charsets.UTF_8)) }
            }
            try {
                val status = call.responseCode
                val stream = if (status < HttpURLConnection.HTTP_BAD_REQUEST) call.inputStream else call.errorStream
                Fetched(status, stream?.bufferedReader()?.use(::readStart).orEmpty())
            } finally {
                call.disconnect()
            }
        } catch (e: IOException) {
            null
        } catch (e: URISyntaxException) {
            null
        } catch (e: IllegalArgumentException) {
            null
        }

        /**
         * What a health check answered [status] and [body] says: mez's own
         * answer is a 200 that starts with [MEZ_HEALTH], and names the venue
         * if the PC is new enough to (ADR-416) and has one.
         */
        fun heard(status: Int, body: String): Mez? {
            if (status != HttpURLConnection.HTTP_OK || !body.startsWith(MEZ_HEALTH)) return null
            return Mez(VENUE.find(body)?.groupValues?.get(1))
        }

        // Up to [READ_CHARS] of the answer: a read may return less than the
        // whole of it, so it reads until the end, the limit, or [TIMEOUT_MS]
        // in all. The read timeout is per read: an answer dripped a little at
        // a time would hold the probe's one thread for minutes.
        private fun readStart(body: Reader): String {
            val start = CharArray(READ_CHARS)
            val until = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(TIMEOUT_MS.toLong())
            var read = 0
            while (read < READ_CHARS && System.nanoTime() < until) {
                val n = body.read(start, read, READ_CHARS - read)
                if (n < 0) break
                read += n
            }
            return String(start, 0, read)
        }
    }
}

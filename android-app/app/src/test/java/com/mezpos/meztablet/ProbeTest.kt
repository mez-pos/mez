package com.mezpos.meztablet

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.InetAddress
import java.net.ServerSocket
import java.net.SocketException
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.thread

class ProbeTest {

    private val servers = mutableListOf<Pc>()

    @After
    fun closeServers() = servers.forEach(Pc::close)

    @Test
    fun `mez is up where its health check answers 200`() {
        assertNotNull(Probe.ask(pc(200).health))
    }

    @Test
    fun `something else at the address is not mez`() {
        assertNull("a 200 that is not mez's answer", Probe.ask(pc(200, body = "<html>ok</html>").health))
        assertNull("a 200 with nothing in it", Probe.ask(pc(200, body = "").health))
        assertNull("a 404", Probe.ask(pc(404).health))
        assertNull("a redirect elsewhere", Probe.ask(pc(302).health))
        assertNull("a 500", Probe.ask(pc(500).health))
    }

    @Test
    fun `mez names the venue it serves`() {
        assertEquals(Mez("ven_7K3M9Q"), Probe.ask(pc(200, body = AT_VENUE).health))
        assertEquals("a mez from before ADR-416", Mez(null), Probe.ask(pc(200).health))
        assertNull("not mez", Probe.ask(pc(404).health))
    }

    @Test
    fun `an answer that arrives in pieces is read whole`() {
        assertEquals(Mez("ven_7K3M9Q"), Probe.ask(pc(200, body = AT_VENUE, pieces = 3).health))
    }

    @Test
    fun `what the health check said, read`() {
        assertEquals(Mez("ven_7K3M9Q"), Probe.heard(200, AT_VENUE))
        assertEquals("before setup", Mez(null), Probe.heard(200, MEZ))
        assertEquals("a venue far into the answer", Mez("ven_1"), Probe.heard(200, "{\"status\":\"ok\",\"x\":\"${"y".repeat(900)}\",\"venue\":\"ven_1\"}"))
        assertEquals("not a public id", Mez(null), Probe.heard(200, "{\"status\":\"ok\",\"venue\":\"ven 1\"}"))
        assertEquals("not a string", Mez(null), Probe.heard(200, "{\"status\":\"ok\",\"venue\":7}"))
        assertNull("another answer", Probe.heard(200, "{\"status\":\"down\",\"venue\":\"ven_1\"}"))
        assertNull("not a 200", Probe.heard(503, AT_VENUE))
    }

    @Test
    fun `nothing at the address`() {
        val gone = pc(200).also(Pc::close)
        assertNull(Probe.ask(gone.health))
        assertNull("not an address", Probe.ask("http://"))
    }

    @Test
    fun `it asks until mez answers, then says so once`() {
        val pc = pc(503)
        val told = CountDownLatch(1)
        val times = AtomicInteger()
        val probe = Probe(every = 20) { times.incrementAndGet(); told.countDown() }
        try {
            probe.start(pc.health, after = 0)
            pc.waitForAsks(2)
            pc.status = 200
            assertTrue("never told", told.await(5, TimeUnit.SECONDS))
            Thread.sleep(200)
            assertEquals(1, times.get())
        } finally {
            probe.close()
        }
    }

    @Test
    fun `stopped, it neither asks nor tells`() {
        val pc = pc(200)
        val times = AtomicInteger()
        val probe = Probe(every = 20) { times.incrementAndGet() }
        try {
            probe.start(pc.health, after = 300)
            probe.stop()
            Thread.sleep(600)
            assertEquals("told", 0, times.get())
            assertEquals("asked", 0, pc.asks.get())
        } finally {
            probe.close()
        }
    }

    @Test
    fun `started again, only the new asking tells`() {
        val first = pc(503)
        val second = pc(200)
        val told = CountDownLatch(1)
        val times = AtomicInteger()
        val probe = Probe(every = 20) { times.incrementAndGet(); told.countDown() }
        try {
            probe.start(first.health, after = 0)
            first.waitForAsks(1)
            probe.start(second.health, after = 0)
            first.status = 200
            assertTrue(told.await(5, TimeUnit.SECONDS))
            Thread.sleep(200)
            assertEquals(1, times.get())
        } finally {
            probe.close()
        }
    }

    private fun pc(status: Int, body: String = MEZ, pieces: Int = 1) = Pc(status, body, pieces).also(servers::add)

    /**
     * A PC on this machine, answering every request with [status] and [body],
     * the body sent in [pieces] with a pause between each.
     */
    private class Pc(@Volatile var status: Int, private val body: String, private val pieces: Int) {
        private val socket = ServerSocket(0, 50, InetAddress.getLoopbackAddress())
        val health = "http://127.0.0.1:${socket.localPort}$HEALTH"
        val asks = AtomicInteger()

        init {
            thread(isDaemon = true) {
                try {
                    while (true) socket.accept().use { call ->
                        // The request line and headers, up to the blank line.
                        val reader = call.getInputStream().bufferedReader()
                        while (reader.readLine()?.isNotEmpty() == true) Unit
                        asks.incrementAndGet()
                        val out = call.getOutputStream()
                        out.write(
                            ("HTTP/1.1 $status X\r\nContent-Length: ${body.length}\r\n" +
                                "Location: http://example.com/\r\nConnection: close\r\n\r\n").toByteArray(),
                        )
                        body.chunked(maxOf(1, (body.length + pieces - 1) / pieces)).forEach { piece ->
                            out.write(piece.toByteArray())
                            out.flush()
                            if (pieces > 1) Thread.sleep(PAUSE_MS)
                        }
                    }
                } catch (e: SocketException) {
                    // Closed.
                }
            }
        }

        fun waitForAsks(n: Int) {
            val until = System.nanoTime() + TimeUnit.SECONDS.toNanos(5)
            while (asks.get() < n && System.nanoTime() < until) Thread.sleep(5)
            check(asks.get() >= n) { "asked ${asks.get()} times, not $n" }
        }

        fun close() = socket.close()
    }

    private companion object {
        // What internal/api/health.go answers, cut short.
        const val MEZ = "{\"status\":\"ok\",\"version\":\"v0.2.0\",\"checks\":{\"db\":\"ok\"}}"

        // Between the pieces of an answer sent in pieces.
        const val PAUSE_MS = 50L

        // The same, from a PC that names its venue (ADR-416).
        const val AT_VENUE = "{\"status\":\"ok\",\"venue\":\"ven_7K3M9Q\",\"version\":\"v0.2.0\"}"
    }
}

package com.mezpos.meztablet

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
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
        assertTrue(Probe.answers(pc(200).health))
    }

    @Test
    fun `something else at the address is not mez`() {
        assertFalse("a 200 that is not mez's answer", Probe.answers(pc(200, body = "<html>ok</html>").health))
        assertFalse("a 200 with nothing in it", Probe.answers(pc(200, body = "").health))
        assertFalse("a 404", Probe.answers(pc(404).health))
        assertFalse("a redirect elsewhere", Probe.answers(pc(302).health))
        assertFalse("a 500", Probe.answers(pc(500).health))
    }

    @Test
    fun `nothing at the address`() {
        val gone = pc(200).also(Pc::close)
        assertFalse(Probe.answers(gone.health))
        assertFalse("not an address", Probe.answers("http://"))
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

    private fun pc(status: Int, body: String = MEZ) = Pc(status, body).also(servers::add)

    /** A PC on this machine, answering every request with [status] and [body]. */
    private class Pc(@Volatile var status: Int, private val body: String) {
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
                        call.getOutputStream().write(
                            ("HTTP/1.1 $status X\r\nContent-Length: ${body.length}\r\n" +
                                "Location: http://example.com/\r\nConnection: close\r\n\r\n$body").toByteArray(),
                        )
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
    }
}

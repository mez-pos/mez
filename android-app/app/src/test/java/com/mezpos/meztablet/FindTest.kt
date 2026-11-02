package com.mezpos.meztablet

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.ByteArrayOutputStream
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.SocketException
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.thread

class FindTest {

    private val sockets = mutableListOf<DatagramSocket>()

    @After
    fun close() = sockets.forEach(DatagramSocket::close)

    @Test
    fun `the PC's address moved, its port and path kept`() {
        assertEquals("http://192.168.1.45:7777/kitchen", movedTo("http://192.168.1.20:7777/kitchen", "192.168.1.45"))
        assertEquals("http://192.168.1.45/", movedTo("http://192.168.1.20/", "192.168.1.45"))
        assertNull(movedTo(AppPages.page(AppPages.CHECKING), "192.168.1.45"))
    }

    @Test
    fun `the question goes to the group unless a debug build names a responder`() {
        assertEquals(MDNS_GROUP, mdnsTo(""))
        assertEquals(InetSocketAddress(InetAddress.getByAddress(byteArrayOf(10, 0, 2, 2)), 5354), mdnsTo("10.0.2.2:5354"))
        assertEquals("a name", MDNS_GROUP, mdnsTo("mez.local:5354"))
        assertEquals("no port", MDNS_GROUP, mdnsTo("10.0.2.2"))
        assertEquals("a port too far", MDNS_GROUP, mdnsTo("10.0.2.2:70000"))
        assertEquals("224.0.0.251:5353", "${MDNS_GROUP.address.hostAddress}:${MDNS_GROUP.port}")
    }

    @Test
    fun `the PC is the address whose mez names the kept venue`() {
        val asked = mutableListOf<String>()
        val found = lookFor(KEPT, VENUE, listOf("192.168.1.20", "8.8.8.8", "192.168.1.50", "192.168.1.45")) { health ->
            asked += health
            when {
                "192.168.1.50" in health -> Mez("ven_DEM0")
                "192.168.1.45" in health -> Mez(VENUE)
                else -> null
            }
        }
        assertEquals("http://192.168.1.45:7777/kitchen", found?.pc)
        assertEquals(App.KITCHEN, found?.app)
        assertEquals(
            "the kept address and one off the network are never asked",
            listOf("http://192.168.1.50:7777/api/v1/health", "http://192.168.1.45:7777/api/v1/health"),
            asked,
        )
    }

    @Test
    fun `a few answers are asked, the tablet's own network first`() {
        val asked = mutableListOf<String>()
        val found = (1..20).map { "10.9.0.$it" } + "192.168.1.45"
        val near = nearFirst(found, own = listOf("192.168.1.31"))
        assertEquals("192.168.1.45", near.first())
        lookFor(KEPT, VENUE, near) { health ->
            asked += health
            null
        }
        assertEquals("never more than four", 4, asked.size)
        assertTrue(asked.first().contains("192.168.1.45"))
    }

    @Test
    fun `no PC found`() {
        assertNull("nobody answered", lookFor(KEPT, VENUE, emptyList()) { Mez(VENUE) })
        assertNull("only another venue", lookFor(KEPT, VENUE, listOf("192.168.1.50")) { Mez("ven_DEM0") })
        assertNull("a mez that does not say", lookFor(KEPT, VENUE, listOf("192.168.1.45")) { Mez(null) })
        assertNull("only the kept address", lookFor(KEPT, VENUE, listOf("192.168.1.20")) { Mez(VENUE) })
    }

    @Test
    fun `the finder asks once and hears every answer`() {
        val asked = AtomicInteger()
        val responder = responder { query, reply ->
            asked.incrementAndGet()
            val id = ((query[0].toInt() and 0xFF) shl 8) or (query[1].toInt() and 0xFF)
            reply(answer(id, 192, 168, 1, 45))
            reply(answer(id, 10, 0, 0, 9))
            reply(answer(id + 1, 172, 16, 0, 1)) // another question's: not heard
        }
        val found = Finder(to = responder, waitMs = 500).find()
        assertEquals(listOf("192.168.1.45", "10.0.0.9"), found)
        assertEquals(1, asked.get())
    }

    @Test
    fun `the finder hears nothing from nobody`() {
        val silent = responder { _, _ -> }
        val started = System.nanoTime()
        assertEquals(emptyList<String>(), Finder(to = silent, waitMs = 300).find())
        assertTrue("it waits no longer than told", System.nanoTime() - started < TimeUnit.SECONDS.toNanos(2))
    }

    @Test
    fun `the ticker ticks until stopped`() {
        val ticks = AtomicInteger()
        val three = CountDownLatch(3)
        val ticker = Ticker(every = 20) {
            ticks.incrementAndGet()
            three.countDown()
        }
        try {
            ticker.start()
            assertTrue("never ticked three times", three.await(5, TimeUnit.SECONDS))
            ticker.stop()
            val stopped = ticks.get()
            Thread.sleep(150)
            assertTrue("ticked after it stopped", ticks.get() <= stopped + 1)
        } finally {
            ticker.close()
        }
    }

    /** A responder on this machine that hears a query and may answer it, more than once. */
    private fun responder(answer: (ByteArray, (ByteArray) -> Unit) -> Unit): InetSocketAddress {
        val socket = DatagramSocket(0, InetAddress.getLoopbackAddress()).also(sockets::add)
        thread(isDaemon = true) {
            val buffer = ByteArray(512)
            try {
                while (true) {
                    val packet = DatagramPacket(buffer, buffer.size)
                    socket.receive(packet)
                    val query = packet.data.copyOf(packet.length)
                    answer(query) { reply -> socket.send(DatagramPacket(reply, reply.size, packet.socketAddress)) }
                }
            } catch (e: SocketException) {
                // Closed.
            }
        }
        return InetSocketAddress(InetAddress.getLoopbackAddress(), socket.localPort)
    }

    private companion object {
        const val KEPT = "http://192.168.1.20:7777/kitchen"
        const val VENUE = "ven_7K3M9Q"

        // A response to the question for mez.local with one A record.
        fun answer(id: Int, vararg address: Int): ByteArray = ByteArrayOutputStream().apply {
            val head = intArrayOf(id shr 8 and 0xFF, id and 0xFF, 0x84, 0, 0, 1, 0, 1, 0, 0, 0, 0)
            head.forEach(::write)
            mdnsQuery(0).drop(12).forEach { write(it.toInt()) } // the question again
            intArrayOf(0xC0, 12, 0, 1, 0x80, 1, 0, 0, 0, 120, 0, 4).forEach(::write)
            address.forEach(::write)
        }.toByteArray()
    }
}

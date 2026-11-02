package com.mezpos.meztablet

import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.InetAddress
import java.net.ServerSocket
import java.net.SocketException
import java.net.URLDecoder
import kotlin.concurrent.thread

class ProofTest {

    private val servers = mutableListOf<ServerSocket>()

    @After
    fun closeServers() = servers.forEach(ServerSocket::close)

    @Test
    fun `the proof is what the PC holding the pairing answers`() {
        // Made outside the app and the server (Python's hmac); the server's
        // TestProofMessageVector holds the same.
        val message = proofMessage("dev_7K3M9Q2A", NONCE, "192.168.1.45:7777")
        assertEquals("0c74779a56dbe468fcfe51165e054306b51b0c28d7385ec47de8992f8d542adf", proofFor(TOKEN, message))
    }

    @Test
    fun `the pairings read at the old address`() {
        val held = heldOf("\"dev_7K3M9Q2A $TOKEN\"")
        assertEquals(listOf("dev_7K3M9Q2A"), held?.map(Held::device))
        assertEquals(listOf(TOKEN), held?.map(Held::token))
        assertFalse("the token is never in a log line", held.toString().contains(TOKEN))
        val other = "nfd_" + "B".repeat(43)
        assertEquals(
            "the till's and the back office's: every one carried is proved",
            listOf("dev_1", "dev_2"),
            heldOf("\"dev_1 $TOKEN,dev_2 $other\"")?.map(Held::device),
        )
        assertNull("no pairing there", heldOf("null"))
        assertNull("not a device", heldOf("\"stf_7K3M9Q2A $TOKEN\""))
        assertNull("not a device's token", heldOf("\"dev_7K3M9Q2A nfs_${"A".repeat(43)}\""))
        assertNull("a token cut short", heldOf("\"dev_7K3M9Q2A nfd_AAAA\""))
        assertNull("more after it", heldOf("\"dev_7K3M9Q2A $TOKEN x\""))
        assertNull("an empty pairing in the list", heldOf("\"dev_1 $TOKEN,\""))
        assertNull("not a string", heldOf("dev_7K3M9Q2A $TOKEN"))
    }

    @Test
    fun `the address asked, as the PC compares it`() {
        assertEquals("192.168.1.45:7777", atOf("http://192.168.1.45:7777/kitchen"))
        assertEquals("the scheme's own port, written", "192.168.1.45:80", atOf("http://192.168.1.45/"))
        assertEquals("192.168.1.45:443", atOf("https://192.168.1.45/"))
        assertNull("a name is not an address the PC can compare", atOf("http://mez.local:7777/"))
        assertNull("not an address", atOf("http://999.1.1.1:7777/"))
        assertNull("a leading zero, which the PC reads otherwise", atOf("http://010.0.0.1:7777/"))
        assertNull(atOf(AppPages.page(AppPages.CHECKING)))
    }

    @Test
    fun `where the proof is asked`() {
        assertEquals(
            "http://192.168.1.45:7777/api/v1/devices/dev_1/proof?nonce=$NONCE&at=192.168.1.45%3A7777",
            proofUrl("http://192.168.1.45:7777/kitchen", "dev_1", NONCE, "192.168.1.45:7777"),
        )
    }

    @Test
    fun `the proof in the PC's answer`() {
        val hex = "0c74779a56dbe468fcfe51165e054306b51b0c28d7385ec47de8992f8d542adf"
        assertEquals(hex, proofIn(200, "{\"proof\":\"$hex\"}"))
        assertNull("not a 200", proofIn(404, "{\"proof\":\"$hex\"}"))
        assertNull("not hex", proofIn(200, "{\"proof\":\"${hex.uppercase()}\"}"))
        assertNull("cut short", proofIn(200, "{\"proof\":\"${hex.take(60)}\"}"))
        assertNull("something else", proofIn(200, "{\"status\":\"ok\"}"))
    }

    @Test
    fun `a PC that holds the pairing proves it`() {
        val pc = pc { device, nonce, at -> proofFor(TOKEN, proofMessage(device, nonce, at)) }
        assertTrue(Proof.proves(pc, Held("dev_1", TOKEN)))
    }

    @Test
    fun `a PC that does not, or answers for another address, does not`() {
        val other = "nfd_" + "B".repeat(43)
        assertFalse("another device's pairing", Proof.proves(pc { d, n, a -> proofFor(other, proofMessage(d, n, a)) }, Held("dev_1", TOKEN)))
        // A device on the wifi passing the question on gets the PC's answer
        // for the PC's own address.
        assertFalse("another address", Proof.proves(pc { d, n, _ -> proofFor(TOKEN, proofMessage(d, n, "192.168.1.45:7777")) }, Held("dev_1", TOKEN)))
        assertFalse("no answer", Proof.proves(pc { _, _, _ -> null }, Held("dev_1", TOKEN)))
    }

    /**
     * A PC on this machine answering the proof with what [answer] makes of the
     * device, nonce and address asked; a null answer is a 404.
     */
    private fun pc(answer: (String, String, String) -> String?): String {
        val socket = ServerSocket(0, 50, InetAddress.getLoopbackAddress()).also(servers::add)
        thread(isDaemon = true) {
            try {
                while (true) socket.accept().use { call ->
                    val reader = call.getInputStream().bufferedReader()
                    val target = reader.readLine().orEmpty().split(' ').getOrNull(1).orEmpty()
                    while (reader.readLine()?.isNotEmpty() == true) Unit
                    val device = target.substringAfter("/devices/").substringBefore('/')
                    val query = target.substringAfter('?').split('&').associate {
                        it.substringBefore('=') to URLDecoder.decode(it.substringAfter('='), "UTF-8")
                    }
                    val proof = answer(device, query["nonce"].orEmpty(), query["at"].orEmpty())
                    val body = if (proof == null) "{}" else "{\"proof\":\"$proof\"}"
                    val status = if (proof == null) "404 Not Found" else "200 OK"
                    call.getOutputStream().write(
                        "HTTP/1.1 $status\r\nContent-Length: ${body.length}\r\nConnection: close\r\n\r\n$body".toByteArray(),
                    )
                }
            } catch (e: SocketException) {
                // Closed.
            }
        }
        return "http://127.0.0.1:${socket.localPort}/"
    }

    private companion object {
        val TOKEN = "nfd_" + "A".repeat(43)
        const val NONCE = "0123456789abcdef0123456789abcdef"
    }
}

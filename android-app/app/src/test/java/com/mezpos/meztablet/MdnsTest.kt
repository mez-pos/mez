package com.mezpos.meztablet

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Test
import java.io.ByteArrayOutputStream

class MdnsTest {

    @Test
    fun `the question for mez local`() {
        val expected = bytes(
            0x12, 0x34, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, // id, flags, one question
            3, 'm'.code, 'e'.code, 'z'.code, 5, 'l'.code, 'o'.code, 'c'.code, 'a'.code, 'l'.code, 0,
            0, 1, 0, 1, // A, IN
        )
        assertArrayEquals(expected, mdnsQuery(0x1234))
    }

    @Test
    fun `the PC's address in its answer`() {
        // As pion/mdns answers a legacy query: the id, the question again,
        // the answer's name a pointer to the question's, the cache-flush bit.
        assertEquals(listOf("192.168.1.45"), mdnsAddresses(answer(0x1234, listOf(a(POINTER, 192, 168, 1, 45))), id = 0x1234))
    }

    @Test
    fun `every address it answers, once each`() {
        val packet = answer(
            7,
            listOf(a(NAME, 192, 168, 1, 45), a(POINTER, 10, 0, 0, 9), a(POINTER, 192, 168, 1, 45)),
        )
        assertEquals(listOf("192.168.1.45", "10.0.0.9"), mdnsAddresses(packet, id = 7))
    }

    @Test
    fun `only mez local's addresses, and only IPv4 ones`() {
        val other = record(name("printer", "local"), type = 1, data = bytes(192, 168, 1, 50))
        val aaaa = record(POINTER, type = 28, data = ByteArray(16))
        assertEquals(listOf("192.168.1.45"), mdnsAddresses(answer(7, listOf(other, aaaa, a(POINTER, 192, 168, 1, 45))), id = 7))
    }

    @Test
    fun `the name is matched whatever its case`() {
        val upper = record(name("MEZ", "Local"), type = 1, data = bytes(192, 168, 1, 45))
        assertEquals(listOf("192.168.1.45"), mdnsAddresses(answer(7, listOf(upper)), id = 7))
    }

    @Test
    fun `nothing from what is not this answer`() {
        val good = answer(7, listOf(a(POINTER, 192, 168, 1, 45)))
        assertEquals("another question's id", emptyList<String>(), mdnsAddresses(good, id = 8))
        val asked = good.copyOf().also { it[2] = 0 } // the response bit off: a question
        assertEquals("a question", emptyList<String>(), mdnsAddresses(asked, id = 7))
        for (cut in good.indices) {
            // Cut anywhere, it is read as far as it goes and never past.
            mdnsAddresses(good.copyOf(cut), id = 7)
        }
        assertEquals("cut short", emptyList<String>(), mdnsAddresses(good, length = good.size - 1, id = 7))
        // The first record starts at 27 (12 for the header, 15 for the
        // question): a name there that points at itself is no name.
        val loop = answer(7, listOf(record(bytes(0xC0, 27), type = 1, data = bytes(1, 2, 3, 4))))
        assertEquals("a loop", emptyList<String>(), mdnsAddresses(loop, id = 7))
    }

    @Test
    fun `a name longer than DNS allows is no name`() {
        val long = name(*Array(130) { "a" }) // 260 characters with its dots
        val packet = answer(7, listOf(record(long, type = 1, data = bytes(1, 2, 3, 4)), a(POINTER, 192, 168, 1, 45)))
        assertEquals(emptyList<String>(), mdnsAddresses(packet, id = 7))
    }

    private companion object {
        val NAME = name("mez", "local")

        // The question's name sits at offset 12, after the header.
        val POINTER = bytes(0xC0, 12)

        fun bytes(vararg values: Int) = ByteArray(values.size) { values[it].toByte() }

        fun name(vararg labels: String): ByteArray = ByteArrayOutputStream().apply {
            for (label in labels) {
                write(label.length)
                write(label.toByteArray())
            }
            write(0)
        }.toByteArray()

        fun record(owner: ByteArray, type: Int, data: ByteArray): ByteArray = ByteArrayOutputStream().apply {
            write(owner)
            write(bytes(0, type, 0x80, 1, 0, 0, 0, 120, 0, data.size)) // class IN with cache-flush, TTL 120
            write(data)
        }.toByteArray()

        fun a(owner: ByteArray, vararg address: Int) = record(owner, type = 1, data = bytes(*address))

        fun answer(id: Int, records: List<ByteArray>): ByteArray = ByteArrayOutputStream().apply {
            write(bytes(id shr 8, id and 0xFF, 0x84, 0, 0, 1, 0, records.size, 0, 0, 0, 0))
            write(NAME)
            write(bytes(0, 1, 0, 1))
            records.forEach(::write)
        }.toByteArray()
    }
}

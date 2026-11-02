package com.mezpos.meztablet

import java.io.ByteArrayOutputStream

/*
 * `mez.local` asked of the venue's network by one multicast DNS query
 * (ADR-416): a legacy query, sent from an ordinary port (RFC 6762 6.7), which
 * the PC's announcer (pion/mdns) answers by unicast to the asker. The
 * packet's making and reading are here, and pure; Finder sends it.
 */

/** The name the PC answers for on its network (ADR-379). */
const val MDNS_NAME = "mez.local"

private const val HEADER = 12
private const val TYPE_A = 1
private const val CLASS_IN = 1
private const val RESPONSE = 0x8000
private const val RCODE = 0x000F

// A record's class carries mDNS's cache-flush bit on top.
private const val CLASS_BITS = 0x7FFF
private const val POINTER = 0xC0
private const val LABEL_BITS = 0x3F
private const val IPV4_BYTES = 4

// A name that takes more jumps than this is a loop, not a name; one longer
// than this is not a DNS name either.
private const val MAX_JUMPS = 16
private const val MAX_NAME = 255

/** The question for [name]'s IPv4 address, with [id] (16 bits), as one packet. */
fun mdnsQuery(id: Int, name: String = MDNS_NAME): ByteArray = ByteArrayOutputStream().apply {
    // Id; flags (a standard query); one question; no answers or records.
    intArrayOf(id, 0, 1, 0, 0, 0).forEach { write16(it) }
    for (label in name.split('.')) {
        val bytes = label.toByteArray(Charsets.US_ASCII)
        write(bytes.size)
        write(bytes)
    }
    write(0)
    write16(TYPE_A)
    write16(CLASS_IN)
}.toByteArray()

/**
 * The IPv4 addresses [packet], its first [length] bytes, answers for [name]:
 * a response to the question [id], each address once, in the order given.
 * What is not such a response gives none; what is cut short gives the
 * records read whole before the cut.
 */
fun mdnsAddresses(packet: ByteArray, length: Int = packet.size, id: Int, name: String = MDNS_NAME): List<String> {
    val read = Packet(packet, minOf(length, packet.size))
    val found = linkedSetOf<String>()
    try {
        val flags = read.u16(2)
        if (read.u16(0) != id || flags and RESPONSE == 0 || flags and RCODE != 0) return emptyList()
        var at = HEADER
        repeat(read.u16(4)) { at = read.name(at).second + 4 } // the questions, again
        val records = read.u16(6) + read.u16(8) + read.u16(10)
        repeat(records) {
            val (owner, next) = read.name(at)
            val type = read.u16(next)
            val cls = read.u16(next + 2) and CLASS_BITS
            val size = read.u16(next + 8)
            val data = next + 10
            read.u8(data + size - 1) // the whole record is in the packet
            if (type == TYPE_A && cls == CLASS_IN && size == IPV4_BYTES && owner.equals(name, ignoreCase = true)) {
                found += (0 until IPV4_BYTES).joinToString(".") { read.u8(data + it).toString() }
            }
            at = data + size
        }
    } catch (e: Malformed) {
        // Cut short or written wrong: what was read whole stands.
    }
    return found.toList()
}

private class Malformed : Exception()

/** A DNS packet's bytes, read with every offset checked. */
private class Packet(private val bytes: ByteArray, private val length: Int) {
    fun u8(at: Int): Int {
        if (at < 0 || at >= length) throw Malformed()
        return bytes[at].toInt() and 0xFF
    }

    fun u16(at: Int): Int = (u8(at) shl 8) or u8(at + 1)

    /** The name at [start], and where what follows it starts. */
    fun name(start: Int): Pair<String, Int> {
        val labels = mutableListOf<String>()
        var at = start
        var next = -1
        var jumps = 0
        while (true) {
            val size = u8(at)
            when {
                size == 0 -> {
                    if (next < 0) next = at + 1
                    return labels.joinToString(".") to next
                }
                size and POINTER == POINTER -> {
                    if (++jumps > MAX_JUMPS) throw Malformed()
                    if (next < 0) next = at + 2
                    at = ((size and LABEL_BITS) shl 8) or u8(at + 1)
                }
                size and POINTER != 0 -> throw Malformed()
                else -> {
                    u8(at + size)
                    labels += String(bytes, at + 1, size, Charsets.US_ASCII)
                    if (labels.sumOf { it.length + 1 } > MAX_NAME + 1) throw Malformed()
                    at += 1 + size
                }
            }
        }
    }
}

private fun ByteArrayOutputStream.write16(value: Int) {
    write(value shr 8 and 0xFF)
    write(value and 0xFF)
}

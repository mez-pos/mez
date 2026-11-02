package com.mezpos.meztablet

import java.io.IOException
import java.net.DatagramPacket
import java.net.Inet4Address
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.MulticastSocket
import java.net.NetworkInterface
import java.net.SocketException
import java.net.SocketTimeoutException
import java.net.URI
import java.net.URISyntaxException
import java.security.SecureRandom
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.TimeUnit

/*
 * The PC found again by itself (ADR-416, Task 5c): when the kept address
 * stops answering, `mez.local` is asked of the network once, each address
 * answered is asked its health, and the one whose mez names the kept venue is
 * where the pairing goes, once it has proved it holds it (ADR-417).
 */

/** Where multicast DNS is asked: the group, at mDNS's port. */
val MDNS_GROUP = InetSocketAddress(InetAddress.getByAddress(byteArrayOf(224.toByte(), 0, 0, 251.toByte())), 5353)

/**
 * Asks [to] (the group, or in a debug build a responder given by address)
 * once who answers for `mez.local`, from an ordinary port, and hears the
 * answers for [waitMs]: the addresses, each once. The group is asked on every
 * network the tablet is on: the PC answers with its address on the network
 * the question came in by, and the default one may not be the venue's.
 * Nothing heard, or a network that refuses, is none.
 */
class Finder(private val to: InetSocketAddress = MDNS_GROUP, private val waitMs: Int = WAIT_MS) {

    fun find(): List<String> {
        val id = random.nextInt(ID_SPACE)
        val found = linkedSetOf<String>()
        try {
            MulticastSocket().use { socket ->
                val query = DatagramPacket(mdnsQuery(id), HEADER_AND_QUESTION, to)
                val ways = if (to.address.isMulticastAddress) networks() else emptyList()
                if (ways.isEmpty()) socket.send(query)
                for (way in ways) {
                    try {
                        socket.networkInterface = way
                        socket.send(query)
                    } catch (e: IOException) {
                        // That network refuses the group: the others are still asked.
                    }
                }
                val until = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(waitMs.toLong())
                val buffer = ByteArray(MAX_PACKET)
                while (true) {
                    val left = TimeUnit.NANOSECONDS.toMillis(until - System.nanoTime())
                    if (left <= 0) break
                    socket.soTimeout = left.toInt()
                    val packet = DatagramPacket(buffer, buffer.size)
                    try {
                        socket.receive(packet)
                    } catch (e: SocketTimeoutException) {
                        break
                    }
                    found += mdnsAddresses(packet.data, packet.length, id)
                }
            }
        } catch (e: IOException) {
            // No network, or one that refuses the group: nothing found.
        } catch (e: SecurityException) {
            // Android 17's local network not allowed: nothing found.
        }
        return found.toList()
    }

    // The networks the tablet is on: up, not its own loopback, able to
    // multicast, with an IPv4 address. None readable: the default one alone.
    private fun networks(): List<NetworkInterface> = try {
        NetworkInterface.getNetworkInterfaces()?.toList().orEmpty().filter { way ->
            way.isUp && !way.isLoopback && way.supportsMulticast() &&
                way.inetAddresses.toList().any { it is Inet4Address }
        }
    } catch (e: SocketException) {
        emptyList()
    }

    companion object {
        /** How long answers are waited for: a PC on the venue's wifi answers in milliseconds. */
        const val WAIT_MS = 1_500

        // The question for mez.local: 12 bytes of header, 11 of name, 4 of type and class.
        private val HEADER_AND_QUESTION = mdnsQuery(0).size

        private const val ID_SPACE = 0x10000
        private const val MAX_PACKET = 1500
        private val random = SecureRandom()
    }
}

/**
 * Where the question goes: the group, or [given], `address:port` of a
 * responder, which a debug build may be given (`-PmezMdns`) to walk on an
 * emulator whose network carries no multicast. Anything else is the group.
 */
fun mdnsTo(given: String): InetSocketAddress {
    val address = given.substringBeforeLast(':', "")
    val port = given.substringAfterLast(':', "").toIntOrNull()
    if (port == null || port !in 1..MAX_PORT || !isIpv4(address)) return MDNS_GROUP
    val bytes = address.split('.').map { it.toInt().toByte() }.toByteArray()
    return InetSocketAddress(InetAddress.getByAddress(bytes), port)
}

private const val MAX_PORT = 65535

/** The host in [url], or null. */
fun hostOf(url: String): String? = try {
    URI(url).host
} catch (e: URISyntaxException) {
    null
}

/** [pc], the PC's kept address, at [address] instead: its scheme, port and path kept. */
fun movedTo(pc: String, address: String): String? {
    if (AppPages.isOwn(pc)) return null
    val uri = try {
        URI(pc)
    } catch (e: URISyntaxException) {
        return null
    }
    if (uri.host.isNullOrEmpty() || uri.scheme == null) return null
    return try {
        URI(uri.scheme, null, address, uri.port, uri.rawPath, null, null).toString()
    } catch (e: URISyntaxException) {
        null
    }
}

/**
 * [found] with the addresses on one of the tablet's own networks ([own],
 * compared by their first three parts) first: the PC on the venue's wifi
 * answers from there, and a device flooding answers rarely does.
 */
fun nearFirst(found: List<String>, own: List<String>): List<String> {
    val nets = own.map { it.substringBeforeLast('.') }.toSet()
    return found.sortedBy { if (it.substringBeforeLast('.') in nets) 0 else 1 }
}

/** The tablet's own IPv4 addresses, for [nearFirst]. */
fun ownAddresses(): List<String> = try {
    NetworkInterface.getNetworkInterfaces()?.toList().orEmpty()
        .filter { it.isUp && !it.isLoopback }
        .flatMap { it.inetAddresses.toList() }
        .filterIsInstance<Inet4Address>()
        .mapNotNull { it.hostAddress }
} catch (e: SocketException) {
    emptyList()
}

/**
 * Where the PC kept at [pc], whose mez named [venue], answers now, among the
 * addresses [found] for `mez.local`: the first, other than the kept one and
 * on the venue's network, whose health [ask] says names [venue]. At most
 * [limit] are asked, each up to Probe.TIMEOUT_MS twice: answers flooded by a
 * device on the wifi cannot hold the watch for long. Null if none does.
 */
fun lookFor(pc: String, venue: String, found: List<String>, limit: Int = MAX_ASKED, ask: (String) -> Mez?): Pairing.Link? {
    val kept = try {
        URI(pc).host
    } catch (e: URISyntaxException) {
        return null
    }
    val candidates = found.distinct().filter { it != kept && onTheNetwork(it) }.take(limit)
    for (address in candidates) {
        val link = movedTo(pc, address)?.let(::readLink) as? Pairing.Link ?: continue
        val health = healthOf(link.pc) ?: continue
        if (ask(health)?.venue == venue) return link
    }
    return null
}

private const val MAX_ASKED = 4

/**
 * Runs [tick] on a thread of its own every [every] ms while started. A tick
 * that throws is told to [failed] and the next one still comes.
 */
class Ticker(
    private val every: Long,
    private val failed: (RuntimeException) -> Unit = {},
    private val tick: () -> Unit,
) {
    private val thread = Executors.newSingleThreadScheduledExecutor { task ->
        Thread(task, "mez-watch").apply { isDaemon = true }
    }

    // Started and stopped from the main thread only.
    private var running: ScheduledFuture<*>? = null

    /** Ticks after [after] ms, then every [every] ms. Started already, nothing changes. */
    fun start(after: Long = every) {
        if (running != null) return
        running = thread.scheduleWithFixedDelay({
            try {
                tick()
            } catch (e: RuntimeException) {
                failed(e)
            }
        }, after, every, TimeUnit.MILLISECONDS)
    }

    /** No more ticks. One already running finishes. */
    fun stop() {
        running?.cancel(false)
        running = null
    }

    /** Stops for good: the activity is gone. */
    fun close() {
        stop()
        thread.shutdownNow()
    }
}

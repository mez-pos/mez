package com.mezpos.meztablet

import java.net.URI
import java.net.URISyntaxException
import java.net.URLEncoder
import java.security.MessageDigest
import java.security.SecureRandom
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec

/*
 * The PC at a new address proves it holds this tablet's pairing before the
 * pairing is carried there (ADR-417). The app reads the device's id and token
 * at the old address, and asks the new one for an HMAC over a nonce and the
 * address asked, keyed by what the PC stores of the token: its SHA-256 in
 * hex. Only the PC holding the pairing can answer, and only for the address
 * the question reached; nothing sent lets anyone answer later.
 */

/** The pairing read at the old address. The token is never in a log line. */
class Held(val device: String, val token: String) {
    override fun toString(): String = "Held($device)"
}

/**
 * The script that reads, at the old address, every pairing it holds (a
 * `mez.<app>.device` that reads as one): `"<device id> <token>,..."`, or
 * null. Every pairing the carry writes is one the new address must prove.
 * A key that does not read as a pairing carries no token. ES5; never throws.
 */
const val READ_HELD = """(function () {
  var held = [];
  try {
    for (var i = 0; i < localStorage.length; i++) {
      var k = localStorage.key(i);
      if (k === null || !/^mez\.[a-z]+\.device${'$'}/.test(k)) continue;
      try {
        var d = JSON.parse(localStorage.getItem(k));
        if (d && typeof d.token === 'string' && d.device && typeof d.device.id === 'string') {
          held.push(d.device.id + ' ' + d.token);
        }
      } catch (e) {
        // Not a pairing: it carries no token.
      }
    }
  } catch (e) {
    return null;
  }
  return held.length ? held.join(',') : null;
})()"""

// A device's public id and its token (`internal/auth`): `nfd_` and 32 bytes
// in unpadded base64url. A tablet holds a few at most: one per app.
private const val PAIR = "dev_[A-Za-z0-9]{1,64} nfd_[A-Za-z0-9_-]{43}"
private val HELD = Regex("\"($PAIR(?:,$PAIR){0,7})\"")

/** The pairings [read] names, as evaluateJavascript hands back [READ_HELD]'s answer; null if none. */
fun heldOf(read: String): List<Held>? {
    val match = HELD.matchEntire(read) ?: return null
    return match.groupValues[1].split(',').map { pair ->
        val (device, token) = pair.split(' ')
        Held(device, token)
    }
}

/** What a proof is made over; the server's `proofMessage` makes the same bytes. */
fun proofMessage(device: String, nonce: String, at: String): ByteArray =
    "mez-proof-v1\n$device\n$nonce\n$at".toByteArray(Charsets.UTF_8)

/**
 * What the PC holding [token]'s pairing answers for [message], in hex:
 * HMAC-SHA256 keyed by the token's SHA-256 in hex, as the PC stores it.
 */
fun proofFor(token: String, message: ByteArray): String {
    val key = MessageDigest.getInstance("SHA-256").digest(token.toByteArray(Charsets.UTF_8)).toHex()
    val mac = Mac.getInstance(HMAC)
    mac.init(SecretKeySpec(key.toByteArray(Charsets.US_ASCII), HMAC))
    return mac.doFinal(message).toHex()
}

private const val HMAC = "HmacSHA256"

/**
 * The address [pc] is asked at, as the PC compares it with its end of the
 * connection: an IPv4 address and port, the port written even when it is the
 * scheme's own. Null for a name: the PC cannot tell it reached a name.
 */
fun atOf(pc: String): String? {
    if (AppPages.isOwn(pc)) return null
    val uri = try {
        URI(pc)
    } catch (e: URISyntaxException) {
        return null
    }
    val host = uri.host ?: return null
    if (!isIpv4(host)) return null
    val port = when {
        uri.port != -1 -> uri.port
        uri.scheme.equals("https", ignoreCase = true) -> HTTPS_PORT
        uri.scheme.equals("http", ignoreCase = true) -> HTTP_PORT
        else -> return null
    }
    return "$host:$port"
}

/** Whether [host] is four parts of 0 to 255 with no leading zero: as the PC's parser reads one. */
fun isIpv4(host: String): Boolean {
    val parts = host.split('.')
    return parts.size == 4 && parts.all { part ->
        part.length in 1..3 && part.all { it in '0'..'9' } && !(part.length > 1 && part.startsWith('0')) &&
            part.toInt() <= MAX_OCTET
    }
}

private const val MAX_OCTET = 255

private const val HTTP_PORT = 80
private const val HTTPS_PORT = 443

/** Where the PC at [pc] is asked to prove [device]'s pairing. */
fun proofUrl(pc: String, device: String, nonce: String, at: String): String? {
    val uri = try {
        URI(pc)
    } catch (e: URISyntaxException) {
        return null
    }
    val scheme = uri.scheme ?: return null
    return "$scheme://$at/api/v1/devices/$device/proof?nonce=$nonce&at=${URLEncoder.encode(at, "UTF-8")}"
}

// The server's answer: HMAC-SHA256 in lowercase hex.
private val PROOF = Regex("\"proof\":\"([0-9a-f]{64})\"")

/** The proof in the PC's answer, [status] and [body], or null. */
fun proofIn(status: Int, body: String): String? =
    if (status == HTTP_OK) PROOF.find(body)?.groupValues?.get(1) else null

private const val HTTP_OK = 200

/** Asking a PC for the proof, on the caller's thread. */
object Proof {
    private const val NONCE_BYTES = 16
    private val random = SecureRandom()

    /** Whether the PC at [pc] proves it holds [held]'s pairing, for the address asked. */
    fun proves(pc: String, held: Held): Boolean {
        val at = atOf(pc) ?: return false
        val nonce = ByteArray(NONCE_BYTES).also(random::nextBytes).toHex()
        val url = proofUrl(pc, held.device, nonce, at) ?: return false
        val answer = Probe.fetch(url) ?: return false
        val proof = proofIn(answer.status, answer.body) ?: return false
        val expected = proofFor(held.token, proofMessage(held.device, nonce, at))
        return MessageDigest.isEqual(proof.toByteArray(Charsets.US_ASCII), expected.toByteArray(Charsets.US_ASCII))
    }
}

private fun ByteArray.toHex(): String = joinToString("") { "%02x".format(it) }

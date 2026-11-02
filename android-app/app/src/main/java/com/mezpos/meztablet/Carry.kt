package com.mezpos.meztablet

import java.net.URI
import java.net.URISyntaxException

/*
 * The device's pairing carried from the PC's old address to its new one
 * (ADR-416). The web app keeps it in the address's localStorage (ADR-338),
 * and a new address is a new origin with none. At the moment of the move the
 * app serves an empty page of its own at each address, never asked of the
 * network, reads the device's keys at the old one and writes them at the new
 * one; then the PC's page opens there, paired. Nothing is written until the
 * new address has proved it holds the pairing (ADR-417, Proof.kt).
 */

/** Where on a PC's address the app serves its empty page, while a carry is under way. */
const val CARRY_PATH = "/mez-app-carry"

/** The app's empty page at [pc]'s address, or null if [pc] is no PC's. */
fun carryPage(pc: String): String? {
    if (AppPages.isOwn(pc)) return null
    val uri = try {
        URI(pc)
    } catch (e: URISyntaxException) {
        return null
    }
    val web = uri.scheme.equals("http", ignoreCase = true) || uri.scheme.equals("https", ignoreCase = true)
    if (!web || uri.host.isNullOrEmpty()) return null
    return URI(uri.scheme, null, uri.host, uri.port, CARRY_PATH, null, null).toString()
}

/** Whether [url] is the app's empty page on some PC's address. */
fun isCarryPage(url: String): Boolean {
    val uri = try {
        URI(url)
    } catch (e: URISyntaxException) {
        return false
    }
    return uri.rawPath == CARRY_PATH && uri.rawQuery == null && carryPage(url) != null
}

/**
 * Whether [url] is [page], the empty page a carry is serving: compared as
 * addresses, since WebView writes one its own way (`:80` left off).
 */
fun atCarryPage(url: String, page: String?): Boolean = page != null && isCarryPage(url) && samePc(url, page)

/**
 * Whether the pairing held at [held], whose mez said [heldVenue], goes to
 * [to], whose mez said [toVenue]: the same venue, at another address. Never
 * when either venue is unknown: another mez on the wifi (a demo laptop) must
 * not be handed the device's token.
 */
fun carries(held: String, heldVenue: String?, to: String, toVenue: String?): Boolean =
    heldVenue != null && heldVenue == toVenue &&
        carryPage(held) != null && carryPage(to) != null && !samePc(held, to)

/**
 * Read at the old address: its `mez.` keys as a JSON object's text, or null
 * when no pairing (`mez.<app>.device`) is among them, and nothing is carried.
 * ES5, and never throws: a page the app serves itself, but a WebView's storage
 * may be refused.
 */
const val READ_KEYS = """(function () {
  try {
    var kept = {}, paired = false;
    for (var i = 0; i < localStorage.length; i++) {
      var k = localStorage.key(i);
      if (k === null || k.indexOf('mez.') !== 0) continue;
      kept[k] = localStorage.getItem(k);
      if (/^mez\.[a-z]+\.device${'$'}/.test(k)) paired = true;
    }
    return paired ? JSON.stringify(kept) : null;
  } catch (e) {
    return null;
  }
})()"""

/**
 * Whether the page's own address holds a pairing (a `mez.<app>.device`):
 * true or false, never the pairing itself. Run on the PC's page, so that the
 * app knows where the pairing in use is held once one is made there.
 */
const val HAS_PAIRING = """(function () {
  try {
    for (var i = 0; i < localStorage.length; i++) {
      var k = localStorage.key(i);
      if (k !== null && /^mez\.[a-z]+\.device${'$'}/.test(k)) return true;
    }
  } catch (e) {
    // Storage refused: nothing held here.
  }
  return false;
})()"""

/**
 * The script that writes, at the new address, what [READ_KEYS] answered:
 * [read] is that answer as evaluateJavascript hands it back, a JSON string.
 * The new address's `mez.` keys give way to the old one's: the address the
 * tablet was using is the device's truth. Nothing is touched until what
 * was read is known to be an object of strings; the old keys are written
 * before the new address's others go, so a write that fails part way
 * leaves the pairing rather than nothing. Null when there is nothing to
 * write. The script answers true once written.
 */
fun writeKeys(read: String): String? {
    if (read.length < MIN_KEYS || !isJsonString(read)) return null
    return """(function (text) {
  try {
    var kept = JSON.parse(text);
    if (kept === null || typeof kept !== 'object') return false;
    var keys = Object.keys(kept).filter(function (k) {
      return k.indexOf('mez.') === 0 && typeof kept[k] === 'string';
    });
    if (keys.length === 0) return false;
    keys.forEach(function (k) { localStorage.setItem(k, kept[k]); });
    var gone = [];
    for (var i = 0; i < localStorage.length; i++) {
      var k = localStorage.key(i);
      if (k !== null && k.indexOf('mez.') === 0 && keys.indexOf(k) < 0) gone.push(k);
    }
    gone.forEach(function (k) { localStorage.removeItem(k); });
    return true;
  } catch (e) {
    return false;
  }
})($read)"""
}

// The shortest answer worth writing: `"{}"`, and READ_KEYS never answers that.
private const val MIN_KEYS = 4

/**
 * Whether [text] is one JSON string and nothing more: a quote, characters
 * with each backslash taking the next, no raw quote or line before the
 * closing quote, and the closing quote last. It goes into a script as is.
 */
fun isJsonString(text: String): Boolean {
    if (text.length < 2 || text.first() != '"') return false
    var i = 1
    while (i < text.length) {
        when (text[i]) {
            '\\' -> i += 2
            '"' -> return i == text.length - 1
            '\n', '\r' -> return false
            else -> i++
        }
    }
    return false
}

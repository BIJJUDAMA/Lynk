// Tests for the cross-platform host metrics abstraction.
// 100% pure ASCII

const test = require('node:test');
const assert = require('node:assert');
const hm = require('./hostmetrics.js');

test('freeMemMb returns positive on this host', () => {
  assert.ok(hm.freeMemMb() > 0);
});

test('timeWaitCount returns null or a non-negative int', () => {
  const v = hm.timeWaitCount();
  assert.ok(v === null || (Number.isInteger(v) && v >= 0));
});

test('ephemeralPortRange returns null or a valid range', () => {
  const r = hm.ephemeralPortRange();
  assert.ok(r === null || (Number.isInteger(r.start) && Number.isInteger(r.end) && r.end > r.start));
});

// Real `ss -s -t` output sample (busybox/iproute2 style: lowercase timewait).
const SS_SUMMARY = [
  'Total: 214',
  'TCP:   41 (estab 12, closed 5, orphaned 0, timewait 6/0)',
  '',
  'Transport Total     IP        IPv6',
  'RAW       0         0         0',
  'UDP       7         5         2',
  'TCP       41        30        11',
].join('\n');

test('parseSsSummaryTimeWait parses lowercase ss summary', () => {
  assert.strictEqual(hm.parseSsSummaryTimeWait(SS_SUMMARY), 6);
});

test('parseSsSummaryTimeWait returns null when token absent', () => {
  assert.strictEqual(hm.parseSsSummaryTimeWait('TCP: 41 (estab 12)'), null);
});

test('parseSsSummaryTimeWait returns null on empty input', () => {
  assert.strictEqual(hm.parseSsSummaryTimeWait(''), null);
  assert.strictEqual(hm.parseSsSummaryTimeWait(null), null);
});

// Real `ss -tan state established` output sample: state column is "ESTAB".
const SS_ESTAB = [
  'State    Recv-Q   Send-Q   Local Address:Port     Peer Address:Port',
  'ESTAB    0        0        10.0.0.5:51234         93.184.216.34:443',
  'ESTAB    0        0        10.0.0.5:51235         93.184.216.34:443',
  'ESTAB    0        0        127.0.0.1:8080         127.0.0.1:52345',
  'ESTAB    0        0        [::1]:8080             [::1]:52111',
  'ESTAB    0        0        10.0.0.5:51236         [2606:4700::1111]:443',
].join('\n');

test('countEstablished counts ESTAB lines and excludes loopback', () => {
  // 5 ESTAB lines total; exclude the 127.0.0.1 pair and the [::1] pair = 3.
  assert.strictEqual(hm.countEstablished(SS_ESTAB), 3);
});

test('countEstablished also accepts full ESTABLISHED token (netstat)', () => {
  const netstat = [
    'tcp        0      0 10.0.0.5:51234     93.184.216.34:443   ESTABLISHED',
    'tcp        0      0 127.0.0.1:8080     127.0.0.1:52345     ESTABLISHED',
    'tcp        0      0 10.0.0.5:22        0.0.0.0:*           LISTEN',
  ].join('\n');
  assert.strictEqual(hm.countEstablished(netstat), 1);
});

test('countEstablished returns null on empty input', () => {
  assert.strictEqual(hm.countEstablished(''), null);
  assert.strictEqual(hm.countEstablished(null), null);
});

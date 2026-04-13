import { beforeEach, test } from 'node:test';
import assert from 'node:assert/strict';
import { __resetSynthCacheForTests, getSynthAudio } from '../synth-cache.js';

let fetchCalls;
let revokeCalls;

beforeEach(() => {
  fetchCalls = [];
  revokeCalls = [];
  global.URL.createObjectURL = blob => `blob:${blob.id}`;
  global.fetch = async (url, options) => {
    fetchCalls.push({ url, options });
    if (options?.signal?.aborted) throw new Error('aborted');
    return {
      ok: true,
      blob: async () => ({ id: fetchCalls.length }),
    };
  };
  global.URL.revokeObjectURL = url => {
    revokeCalls.push(url);
  };
  __resetSynthCacheForTests();
  revokeCalls = [];
});

test('getSynthAudio: caches hits by full synthesis settings', async () => {
  const a = await getSynthAudio('猫', { speaker: 1, speedScale: 1, intonationScale: 1 });
  const b = await getSynthAudio('猫', { speaker: 1, speedScale: 1, intonationScale: 1 });
  const c = await getSynthAudio('猫', { speaker: 2, speedScale: 1, intonationScale: 1 });
  assert.equal(a, b);
  assert.notEqual(a, c);
  assert.equal(fetchCalls.length, 2);
});

test('getSynthAudio: LRU read bumps recency before eviction', async () => {
  for (let i = 0; i < 100; i++) {
    await getSynthAudio(`word-${i}`, {});
  }
  await getSynthAudio('word-0', {});
  await getSynthAudio('word-100', {});
  assert.equal(revokeCalls.includes('blob:2'), true);
  assert.equal(revokeCalls.includes('blob:1'), false);
});

test('getSynthAudio: throws on non-ok response', async () => {
  global.fetch = async () => ({ ok: false, status: 503 });
  await assert.rejects(() => getSynthAudio('猫', {}), /synthesis failed: 503/);
});

test('getSynthAudio: propagates aborted fetch failure', async () => {
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(() => getSynthAudio('猫', {}, controller.signal), /aborted/);
});

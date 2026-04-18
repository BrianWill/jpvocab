import { afterEach, test } from 'node:test';
import assert from 'node:assert/strict';
import { streamBatchAdd } from '../lexicon-add-modal-utils.js';

const originalFetch = global.fetch;

afterEach(() => {
  global.fetch = originalFetch;
});

test('streamBatchAdd: treats empty 204 batch responses as done', async () => {
  let doneCount = 0;
  let rowCount = 0;
  global.fetch = async () => new Response(null, { status: 204 });

  await streamBatchAdd({
    rawWords: '???',
    onDone: () => { doneCount++; },
    onRow: () => { rowCount++; },
  });

  assert.equal(doneCount, 1);
  assert.equal(rowCount, 0);
});

test('streamBatchAdd: treats a clean stream EOF without done event as done', async () => {
  const rows = [];
  let doneCount = 0;
  global.fetch = async () => new Response(
    'data: {"word":"猫","added":false,"reason":"already in lexicon"}\n\n',
    { status: 200 }
  );

  await streamBatchAdd({
    rawWords: '猫',
    onDone: () => { doneCount++; },
    onRow: data => { rows.push(data); },
  });

  assert.deepEqual(rows, [{ word: '猫', added: false, reason: 'already in lexicon' }]);
  assert.equal(doneCount, 1);
});

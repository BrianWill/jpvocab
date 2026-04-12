import { test } from 'node:test';
import assert from 'node:assert/strict';
import { computeVisibleRange, mergeWordPage, removeWordAtIndex } from '../lexicon-virtual.js';

test('computeVisibleRange returns viewport range with overscan', () => {
  assert.deepEqual(
    computeVisibleRange({ scrollTop: 240, viewportHeight: 300, itemHeight: 120, totalItems: 100, overscan: 2 }),
    { start: 0, end: 7 }
  );
});

test('computeVisibleRange clamps to total item count', () => {
  assert.deepEqual(
    computeVisibleRange({ scrollTop: 1080, viewportHeight: 300, itemHeight: 120, totalItems: 10, overscan: 3 }),
    { start: 6, end: 10 }
  );
});

test('mergeWordPage inserts page items at the requested offset', () => {
  const merged = mergeWordPage([{ word: 'A' }, { word: 'B' }], 2, [{ word: 'C' }, { word: 'D' }], 5);
  assert.equal(merged.length, 5);
  assert.deepEqual(merged.map(item => item?.word ?? null), ['A', 'B', 'C', 'D', null]);
});

test('removeWordAtIndex removes one slot and shifts later items left', () => {
  const next = removeWordAtIndex([{ word: 'A' }, { word: 'B' }, { word: 'C' }], 1);
  assert.deepEqual(next.map(item => item.word), ['A', 'C']);
});

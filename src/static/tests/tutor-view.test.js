import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseResponse, responseToHTML } from '../tutor-view.js';

test('parseResponse returns parsed object when content is a JSON object', () => {
  assert.deepEqual(parseResponse('{"jp":"猫","en":"cat"}'), { jp: '猫', en: 'cat' });
});

test('parseResponse falls back to en when content is not a JSON object', () => {
  assert.deepEqual(parseResponse('hello'), { en: 'hello' });
  assert.deepEqual(parseResponse('["x"]'), { en: '["x"]' });
});

test('responseToHTML renders jp first with en tooltip and note last', () => {
  const html = responseToHTML({
    note: 'remember this',
    extra: 'value',
    question: 'How do you say it?',
    en: 'cat',
    jp: '猫',
  });

  const jpIndex = html.indexOf('tutor-seg--jp');
  const questionIndex = html.indexOf('How do you say it?');
  const extraIndex = html.indexOf('tutor-seg--extra');
  const noteIndex = html.indexOf('tutor-seg--note');

  assert.notEqual(jpIndex, -1);
  assert.notEqual(questionIndex, -1);
  assert.notEqual(extraIndex, -1);
  assert.notEqual(noteIndex, -1);
  assert.ok(jpIndex < questionIndex);
  assert.ok(questionIndex < extraIndex);
  assert.ok(extraIndex < noteIndex);
  assert.match(html, /data-tooltip="cat"/);
});

test('responseToHTML escapes HTML-sensitive content', () => {
  const html = responseToHTML({
    jp: '<猫>',
    en: '"cat"',
    extra: '<script>',
  });

  assert.match(html, /&lt;猫&gt;/);
  assert.match(html, /&quot;cat&quot;/);
  assert.match(html, /&lt;script&gt;/);
});

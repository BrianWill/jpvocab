import { test } from 'node:test';
import assert from 'node:assert/strict';
import { addDays, computeAvgForActivity, dayLabel, toDateStr, weekSunday } from '../activity-utils.js';

test('weekSunday: returns the Sunday for a given date', () => {
  assert.equal(toDateStr(weekSunday('2026-04-15')), '2026-04-12');
});

test('addDays and toDateStr: handle month boundaries', () => {
  assert.equal(toDateStr(addDays(new Date('2026-01-31T00:00:00Z'), 1)), '2026-02-01');
});

test('computeAvgForActivity: uses full window denominator including empty days', () => {
  const activityData = {
    '2026-04-10': { drilled: [{}, {}], added: [{}], cleared: [] },
    '2026-04-12': { drilled: [{}], added: [], cleared: [{}] },
  };
  assert.equal(computeAvgForActivity(activityData, '2026-04-10', '2026-04-12', 'drilled', 3), '1.0');
  assert.equal(computeAvgForActivity(activityData, '2026-04-10', '2026-04-12', 'added', null), '0.3');
});

test('dayLabel: uses month/day on Sundays and day number otherwise', () => {
  assert.match(dayLabel('2026-04-12'), /(Apr|12)/);
  assert.equal(dayLabel('2026-04-13'), '13');
});

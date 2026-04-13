import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  buildImageSourceOptionsHtml,
  buildProviderOptionsHtml,
  hasAvailableProvider,
  imageSourceUnavailableTooltip,
  providerUnavailableTooltip,
} from '../generation-ui-utils.js';

const providerModels = [
  { key: 'openai', label: 'OpenAI', envKey: 'OPENAI_API_KEY', models: [['gpt-4.1', 'GPT-4.1']] },
  { key: 'anthropic', label: 'Anthropic', envKey: 'ANTHROPIC_API_KEY', models: [['claude', 'Claude']] },
];

test('hasAvailableProvider: detects enabled providers', () => {
  assert.equal(hasAvailableProvider({ openai: true }, providerModels), true);
  assert.equal(hasAvailableProvider({ openai: false, anthropic: false }, providerModels), false);
});

test('buildProviderOptionsHtml: marks unavailable providers as disabled', () => {
  const html = buildProviderOptionsHtml({ openai: true, anthropic: false }, providerModels);
  assert.match(html, /<optgroup label="OpenAI">/);
  assert.match(html, /<optgroup label="Anthropic — no API key" disabled>/);
});

test('providerUnavailableTooltip: lists missing provider env vars', () => {
  const tooltip = providerUnavailableTooltip({ openai: true, anthropic: false }, providerModels);
  assert.match(tooltip, /ANTHROPIC_API_KEY/);
  assert.match(tooltip, /restart the program/);
});

test('buildImageSourceOptionsHtml and imageSourceUnavailableTooltip: reflect missing keys', () => {
  const html = buildImageSourceOptionsHtml({ unsplash: true, pexels: false, pixabay: false, bing: true });
  assert.match(html, /<option value="wikimedia">Wikimedia<\/option>/);
  assert.match(html, /Pexels — no key/);
  const tooltip = imageSourceUnavailableTooltip({ unsplash: true, pexels: false, pixabay: false, bing: true });
  assert.match(tooltip, /PEXELS_API_KEY/);
  assert.match(tooltip, /PIXABAY_API_KEY/);
});

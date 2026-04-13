import { escapeHtml } from './html-utils.js';

export const IMAGE_SOURCES = [
  { key: 'unsplash', label: 'Unsplash', envKey: 'UNSPLASH_ACCESS_KEY' },
  { key: 'pexels', label: 'Pexels', envKey: 'PEXELS_API_KEY' },
  { key: 'pixabay', label: 'Pixabay', envKey: 'PIXABAY_API_KEY' },
  { key: 'bing', label: 'Bing', envKey: 'BING_API_KEY' },
];

export function hasAvailableProvider(providers, providerModels) {
  return !!(providers && providerModels.some(provider => providers[provider.key]));
}

function missingProviderLines(providers, providerModels) {
  if (!providers) return [];
  return providerModels
    .filter(provider => !providers[provider.key])
    .map(provider => provider.label + ': set ' + provider.envKey + ' to enable');
}

function missingImageSourceLines(imageSources) {
  if (!imageSources) return [];
  return IMAGE_SOURCES
    .filter(source => !imageSources[source.key])
    .map(source => source.label + ': set ' + source.envKey + ' to enable');
}

export function buildProviderOptionsHtml(providers, providerModels) {
  return providerModels.map(({ key, label, models }) => {
    const available = providers && providers[key];
    const groupLabel = available ? label : label + ' — no API key';
    const options = models.map(([value, text]) => (
      '<option value="' + escapeHtml(value) + '">' + escapeHtml(text) + '</option>'
    )).join('');
    return '<optgroup label="' + escapeHtml(groupLabel) + '"' + (available ? '' : ' disabled') + '>' + options + '</optgroup>';
  }).join('');
}

export function buildImageSourceOptionsHtml(imageSources) {
  return '<option value="wikimedia">Wikimedia</option>' +
    IMAGE_SOURCES.map(({ key, label }) => {
      const available = imageSources && imageSources[key];
      return '<option value="' + escapeHtml(key) + '"' + (available ? '' : ' disabled') + '>' +
        escapeHtml(label + (available ? '' : ' — no key')) + '</option>';
    }).join('');
}

export function providerUnavailableTooltip(providers, providerModels) {
  const lines = missingProviderLines(providers, providerModels);
  return lines.length ? lines.join('\n') + '\n— then restart the program' : null;
}

export function imageSourceUnavailableTooltip(imageSources) {
  const lines = missingImageSourceLines(imageSources);
  return lines.length ? lines.join('\n') + '\n— then restart the program' : null;
}

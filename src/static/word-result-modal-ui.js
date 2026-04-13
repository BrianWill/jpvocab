export function buildWordResultRowMarkup({
  data,
  esc,
  typeLabels,
  buildWordResultDetails,
  buildWordResultImage,
  getWordBtnLabel,
  generateType,
  hasProviders,
  removeSymbol = '✕',
  incorrectSymbol = '✗',
}) {
  const badge = data.added
    ? '<span class="result-badge badge-added">added</span>'
    : '<span class="result-badge badge-skipped">' + esc(data.reason) + '</span>';
  const removeBtn =
    '<button class="btn-delete btn-word-remove" data-tooltip="Remove word" data-word="' + esc(data.word) + '">' + removeSymbol + '</button>';
  const generateBtn = data.word_id
    ? '<button class="btn-generate"' +
        (hasProviders ? '' : ' disabled') +
        ' data-tooltip="Uses an AI API request to get the word\'s reading, part-of-speech, meaning, and an example sentence">' +
          getWordBtnLabel(generateType) +
        '</button>'
    : '';

  let inlineExtra;
  if (data.word_id) {
    const correct = data.drill_count ?? 0;
    const incorrect = data.drill_incorrect ?? 0;
    const target = data.drill_target ?? 0;
    inlineExtra =
      '<span class="word-result-drill">' +
        '<span class="word-result-actions">' + generateBtn + removeBtn + '</span>' +
        '<span class="drill-correct" data-tooltip="Times answered correctly">✓ ' + correct + '</span>' +
        '<span class="drill-incorrect" data-tooltip="Times answered incorrectly">' + incorrectSymbol + ' ' + incorrect + '</span>' +
        '<span class="target-stepper" data-tooltip="Remaining drills to target">' +
          '<span class="drill-target-label">🎯</span>' +
          '<span class="drill-target-val" data-target="' + target + '">' + target + '</span>' +
          '<button class="btn-target-adj">−</button>' +
          '<button class="btn-target-adj">+</button>' +
        '</span>' +
      '</span>';
  } else {
    inlineExtra = '<span class="word-result-drill">' + removeBtn + '</span>';
  }

  const details = buildWordResultDetails(data.word, data, typeLabels);
  const imageHtml = buildWordResultImage(data.image_path, '');
  return (
    '<div class="word-result-main"><span class="result-word">' + esc(data.word) + '</span>' + badge + inlineExtra + '</div>' +
    '<div class="word-result-body">' + details + imageHtml + '</div>'
  );
}

export function applyGenerateButtonState(containerEl, { disabled, tooltip, label }) {
  containerEl.querySelectorAll('.btn-generate:not(.btn-generate--busy)').forEach(btn => {
    btn.disabled = disabled;
    btn.dataset.tooltip = tooltip;
    btn.innerHTML = label;
  });
}

export function buildWordResultFooterHtml({
  prefix = '',
  hasProviders,
  optgroupsHtml,
  progTip,
  imageSourceOptions,
  imageSourceTip,
}) {
  return (
    '<select id="' + prefix + 'add-result-model-select" class="add-result-model-select"' + (hasProviders ? '' : ' disabled') + '>' +
      (!hasProviders ? '<option value="" selected>no API keys configured</option>' : '') +
      optgroupsHtml +
    '</select>' +
    (progTip ? '<span class="provider-info-icon" data-tooltip="' + progTip + '">?</span>' : '') +
    '<div id="' + prefix + 'add-result-modal-action" style="margin-left:0.4rem;display:flex;align-items:center;gap:0.4rem"></div>' +
    '<select id="' + prefix + 'add-result-image-source-select" class="add-result-model-select" style="display:none">' +
      imageSourceOptions +
    '</select>' +
    (imageSourceTip ? '<span id="' + prefix + 'add-result-image-source-icon" class="provider-info-icon" style="display:none" data-tooltip="' + imageSourceTip + '">?</span>' : '') +
    '<div id="' + prefix + 'add-result-modal-status" class="modal-status" style="padding:0;border:none;margin-left:auto"></div>' +
    '<button id="' + prefix + 'btn-add-result-remove" class="btn-danger">Remove the added words</button>' +
    '<button id="' + prefix + 'btn-add-result-close" class="btn-save">Close</button>'
  );
}

export function buildWordResultActionHtml({
  pendingGenerates,
  enabled,
  generateType,
  labels,
  tooltip,
  menuId,
  mainButtonClass = '',
  arrowButtonClass = '',
}) {
  if (pendingGenerates > 0) {
    return '<button class="btn-danger btn-generate--cancel"><span class="spinner"></span>Cancel generation</button>';
  }
  return (
    '<div class="split-btn-wrap">' +
      '<button class="btn-save btn-generate--all split-btn-main' + (mainButtonClass ? ' ' + mainButtonClass : '') + '"' +
        (enabled ? '' : ' disabled') +
        (tooltip ? ' data-tooltip="' + tooltip + '"' : '') +
      '>' + labels[generateType] + '</button>' +
      '<button class="btn-save btn-generate--all split-btn-arrow' + (arrowButtonClass ? ' ' + arrowButtonClass : '') + '"' +
        (enabled ? '' : ' disabled') +
      '>▾</button>' +
      '<div class="split-btn-menu" id="' + menuId + '" hidden>' +
        Object.keys(labels).map(type =>
          '<button class="split-btn-option' + (type === generateType ? ' split-btn-option--active' : '') + '" data-type="' + type + '">' +
            labels[type] +
          '</button>'
        ).join('') +
      '</div>' +
    '</div>'
  );
}

export function buildCountsHtml({ addedCount, skippedCount }) {
  const skippedHtml = skippedCount > 0 ? '<span class="status-skipped">' + skippedCount + ' skipped</span>' : '';
  return (
    '<span class="modal-status-counts">' +
      '<span>' + addedCount + ' added</span>' +
      skippedHtml +
    '</span>'
  );
}

export async function loadDrillData() {
  const [wordsResp, kanjiResp, settingsResp, currentSessionResp] = await Promise.all([
    fetch('/api/words'),
    fetch('/api/kanji'),
    fetch('/api/settings/drill'),
    fetch('/api/drill/sessions/current'),
  ]);

  const allWords = await wordsResp.json();
  const kanjiList = await kanjiResp.json();
  const settings = await settingsResp.json();
  const currentSessionData = await currentSessionResp.json();

  return {
    allWords,
    currentSession: currentSessionData.session,
    kanjiList,
    settings,
  };
}

export async function createSession(sessionState) {
  const resp = await fetch('/api/drill/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ state: sessionState }),
  });
  const data = await resp.json();
  return data.id;
}

export async function postAnswer(sessionId, wordId, correct, sessionState) {
  if (!sessionId) return;
  const resp = await fetch('/api/drill/sessions/' + sessionId + '/answers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ wordId, correct, state: sessionState }),
  });
  if (!resp.ok) throw new Error('failed to save drill answer');
}

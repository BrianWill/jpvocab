const placeholder = '<span class="detail-placeholder">- - -</span>';

const words = [
  { word: '走る', reading: 'はしる', meaning: 'to run', type: 'godan-verb', exampleJp: '公園を走る。', exampleEn: '"I run in the park."' },
  { word: '美しい', reading: 'うつくしい', meaning: 'beautiful', type: 'i-adjective', exampleJp: '美しい花が咲いている。', exampleEn: '"Beautiful flowers are blooming."' },
  { word: '話す', reading: 'はなす', meaning: 'to speak; to talk', type: 'godan-verb', exampleJp: '日本語を話す。', exampleEn: '"I speak Japanese."' },
  { word: '静か', reading: 'しずか', meaning: 'quiet', type: 'na-adjective', exampleJp: '図書館は静かだ。', exampleEn: '"The library is quiet."' },
  { word: '始める', reading: 'はじめる', meaning: 'to begin; to start', type: 'ichidan-verb', exampleJp: '仕事を始める。', exampleEn: '"I start work."' },
  { word: '天気', reading: 'てんき', meaning: 'weather', type: 'noun', exampleJp: '今日は天気がいい。', exampleEn: '"The weather is nice today."' },
  { word: '忘れる', reading: 'わすれる', meaning: 'to forget', type: 'ichidan-verb', exampleJp: '名前を忘れる。', exampleEn: '"I forget the name."' },
  { word: '危ない', reading: 'あぶない', meaning: 'dangerous', type: 'i-adjective', exampleJp: 'この道は危ない。', exampleEn: '"This road is dangerous."' },
  { word: '選ぶ', reading: 'えらぶ', meaning: 'to choose; to select', type: 'godan-verb', exampleJp: '好きな色を選ぶ。', exampleEn: '"I choose my favorite color."' },
  { word: '問題', reading: 'もんだい', meaning: 'problem; question', type: 'noun', exampleJp: '問題を解く。', exampleEn: '"I solve the problem."' },
  { word: '暖かい', reading: 'あたたかい', meaning: 'warm', type: 'i-adjective', exampleJp: '今日は暖かい。', exampleEn: '"It is warm today."' },
  { word: '借りる', reading: 'かりる', meaning: 'to borrow', type: 'ichidan-verb', exampleJp: '本を借りる。', exampleEn: '"I borrow a book."' },
  { word: '経験', reading: 'けいけん', meaning: 'experience', type: 'noun', exampleJp: 'いい経験になった。', exampleEn: '"It became a good experience."' },
  { word: '集める', reading: 'あつめる', meaning: 'to collect; to gather', type: 'ichidan-verb', exampleJp: '切手を集める。', exampleEn: '"I collect stamps."' },
  { word: '複雑', reading: 'ふくざつ', meaning: 'complicated; complex', type: 'na-adjective', exampleJp: 'この地図は複雑だ。', exampleEn: '"This map is complicated."' },
  { word: '届く', reading: 'とどく', meaning: 'to arrive; to reach', type: 'godan-verb', exampleJp: '手紙が届く。', exampleEn: '"The letter arrives."' },
  { word: '練習', reading: 'れんしゅう', meaning: 'practice', type: 'noun', exampleJp: '毎日練習する。', exampleEn: '"I practice every day."' },
  { word: '眠い', reading: 'ねむい', meaning: 'sleepy', type: 'i-adjective', exampleJp: 'とても眠い。', exampleEn: '"I am very sleepy."' },
  { word: '伝える', reading: 'つたえる', meaning: 'to convey; to tell', type: 'ichidan-verb', exampleJp: '気持ちを伝える。', exampleEn: '"I convey my feelings."' },
  { word: '丁寧', reading: 'ていねい', meaning: 'polite; careful', type: 'na-adjective', exampleJp: '丁寧に説明する。', exampleEn: '"I explain politely."' },
  { word: '探す', reading: 'さがす', meaning: 'to search; to look for', type: 'godan-verb', exampleJp: '鍵を探す。', exampleEn: '"I look for the key."' },
  { word: '景色', reading: 'けしき', meaning: 'scenery; view', type: 'noun', exampleJp: '景色がきれいだ。', exampleEn: '"The scenery is beautiful."' },
  { word: '壊れる', reading: 'こわれる', meaning: 'to break (intransitive)', type: 'ichidan-verb', exampleJp: '時計が壊れる。', exampleEn: '"The clock breaks."' },
  { word: '残念', reading: 'ざんねん', meaning: 'regrettable; unfortunate', type: 'na-adjective', exampleJp: '残念な結果だった。', exampleEn: '"It was an unfortunate result."' },
  { word: '誘う', reading: 'さそう', meaning: 'to invite', type: 'godan-verb', exampleJp: '友達を誘う。', exampleEn: '"I invite a friend."' },
  { word: '約束', reading: 'やくそく', meaning: 'promise; appointment', type: 'noun', exampleJp: '約束を守る。', exampleEn: '"I keep the promise."' },
  { word: '優しい', reading: 'やさしい', meaning: 'kind; gentle', type: 'i-adjective', exampleJp: '彼は優しい人だ。', exampleEn: '"He is a kind person."' },
  { word: '受ける', reading: 'うける', meaning: 'to receive; to take (exam)', type: 'ichidan-verb', exampleJp: '試験を受ける。', exampleEn: '"I take an exam."' },
  { word: '自由', reading: 'じゆう', meaning: 'freedom; free', type: 'na-adjective', exampleJp: '自由な時間が欲しい。', exampleEn: '"I want free time."' },
  { word: '捨てる', reading: 'すてる', meaning: 'to throw away', type: 'ichidan-verb', exampleJp: 'ゴミを捨てる。', exampleEn: '"I throw away the trash."' },
  // New words
  { word: '泳ぐ', reading: 'およぐ', meaning: 'to swim', type: 'godan-verb', exampleJp: '海で泳ぐ。', exampleEn: '"I swim in the sea."' },
  { word: '運ぶ', reading: 'はこぶ', meaning: 'to carry', type: 'godan-verb', exampleJp: '荷物を運ぶ。', exampleEn: '"I carry the luggage."' },
  { word: '起きる', reading: 'おきる', meaning: 'to wake up; to get up', type: 'ichidan-verb', exampleJp: '早く起きる。', exampleEn: '"I wake up early."' },
  { word: '落ちる', reading: 'おちる', meaning: 'to fall', type: 'ichidan-verb', exampleJp: '葉が落ちる。', exampleEn: '"The leaves fall."' },
  { word: '変える', reading: 'かえる', meaning: 'to change', type: 'ichidan-verb', exampleJp: '計画を変える。', exampleEn: '"I change the plan."' },
  { word: '考える', reading: 'かんがえる', meaning: 'to think; to consider', type: 'ichidan-verb', exampleJp: '答えを考える。', exampleEn: '"I think of the answer."' },
  { word: '決める', reading: 'きめる', meaning: 'to decide', type: 'ichidan-verb', exampleJp: '目標を決める。', exampleEn: '"I decide on a goal."' },
  { word: '聞こえる', reading: 'きこえる', meaning: 'to be audible; to hear', type: 'ichidan-verb', exampleJp: '音楽が聞こえる。', exampleEn: '"I can hear the music."' },
  { word: '比べる', reading: 'くらべる', meaning: 'to compare', type: 'ichidan-verb', exampleJp: '値段を比べる。', exampleEn: '"I compare the prices."' },
  { word: '困る', reading: 'こまる', meaning: 'to be troubled; to be in a fix', type: 'godan-verb', exampleJp: '道に迷って困る。', exampleEn: '"I am troubled after getting lost."' },
  { word: '転ぶ', reading: 'ころぶ', meaning: 'to fall down; to tumble', type: 'godan-verb', exampleJp: '滑って転ぶ。', exampleEn: '"I slip and fall down."' },
  { word: '触る', reading: 'さわる', meaning: 'to touch', type: 'godan-verb', exampleJp: '猫に触る。', exampleEn: '"I touch the cat."' },
  { word: '調べる', reading: 'しらべる', meaning: 'to investigate; to look up', type: 'ichidan-verb', exampleJp: '言葉を調べる。', exampleEn: '"I look up the word."' },
  { word: '信じる', reading: 'しんじる', meaning: 'to believe', type: 'ichidan-verb', exampleJp: '友達を信じる。', exampleEn: '"I believe my friend."' },
  { word: '育てる', reading: 'そだてる', meaning: 'to raise; to bring up', type: 'ichidan-verb', exampleJp: '子供を育てる。', exampleEn: '"I raise a child."' },
  { word: '続ける', reading: 'つづける', meaning: 'to continue', type: 'ichidan-verb', exampleJp: '練習を続ける。', exampleEn: '"I continue practicing."' },
  { word: '手伝う', reading: 'てつだう', meaning: 'to help; to assist', type: 'godan-verb', exampleJp: '母を手伝う。', exampleEn: '"I help my mother."' },
  { word: '直す', reading: 'なおす', meaning: 'to fix; to correct', type: 'godan-verb', exampleJp: '時計を直す。', exampleEn: '"I fix the clock."' },
  { word: '並ぶ', reading: 'ならぶ', meaning: 'to line up; to stand in a row', type: 'godan-verb', exampleJp: '列に並ぶ。', exampleEn: '"I line up in the queue."' },
  { word: '逃げる', reading: 'にげる', meaning: 'to run away; to escape', type: 'ichidan-verb', exampleJp: '危険から逃げる。', exampleEn: '"I escape from danger."' },
  { word: '脱ぐ', reading: 'ぬぐ', meaning: 'to take off (clothes)', type: 'godan-verb', exampleJp: '靴を脱ぐ。', exampleEn: '"I take off my shoes."' },
  { word: '登る', reading: 'のぼる', meaning: 'to climb', type: 'godan-verb', exampleJp: '山に登る。', exampleEn: '"I climb the mountain."' },
  { word: '払う', reading: 'はらう', meaning: 'to pay', type: 'godan-verb', exampleJp: '料金を払う。', exampleEn: '"I pay the fee."' },
  { word: '減る', reading: 'へる', meaning: 'to decrease; to diminish', type: 'godan-verb', exampleJp: '体重が減る。', exampleEn: '"My weight decreases."' },
  { word: '曲がる', reading: 'まがる', meaning: 'to turn; to bend', type: 'godan-verb', exampleJp: '角を曲がる。', exampleEn: '"I turn the corner."' },
  { word: '守る', reading: 'まもる', meaning: 'to protect; to keep (a promise)', type: 'godan-verb', exampleJp: 'ルールを守る。', exampleEn: '"I follow the rules."' },
  { word: '見える', reading: 'みえる', meaning: 'to be visible; can see', type: 'ichidan-verb', exampleJp: '海が見える。', exampleEn: '"I can see the sea."' },
  { word: '向かう', reading: 'むかう', meaning: 'to head toward; to face', type: 'godan-verb', exampleJp: '駅に向かう。', exampleEn: '"I head toward the station."' },
  { word: '戻る', reading: 'もどる', meaning: 'to return; to go back', type: 'godan-verb', exampleJp: '家に戻る。', exampleEn: '"I return home."' },
  { word: '焼く', reading: 'やく', meaning: 'to bake; to grill', type: 'godan-verb', exampleJp: 'パンを焼く。', exampleEn: '"I bake bread."' },
  { word: '意見', reading: 'いけん', meaning: 'opinion', type: 'noun', exampleJp: '意見を言う。', exampleEn: '"I state my opinion."' },
  { word: '機会', reading: 'きかい', meaning: 'opportunity; chance', type: 'noun', exampleJp: '機会を活かす。', exampleEn: '"I make use of the opportunity."' },
  { word: '記念', reading: 'きねん', meaning: 'commemoration; memory', type: 'noun', exampleJp: '記念写真を撮る。', exampleEn: '"I take a commemorative photo."' },
  { word: '計画', reading: 'けいかく', meaning: 'plan', type: 'noun', exampleJp: '計画を立てる。', exampleEn: '"I make a plan."' },
  { word: '原因', reading: 'げんいん', meaning: 'cause; reason', type: 'noun', exampleJp: '事故の原因を調べる。', exampleEn: '"I investigate the cause of the accident."' },
  { word: '事故', reading: 'じこ', meaning: 'accident', type: 'noun', exampleJp: '交通事故が起きた。', exampleEn: '"A traffic accident occurred."' },
  { word: '準備', reading: 'じゅんび', meaning: 'preparation', type: 'noun', exampleJp: '準備ができた。', exampleEn: '"The preparations are done."' },
  { word: '場所', reading: 'ばしょ', meaning: 'place; location', type: 'noun', exampleJp: '待ち合わせの場所を決める。', exampleEn: '"I decide on a meeting place."' },
  { word: '心配', reading: 'しんぱい', meaning: 'worry; concern', type: 'noun', exampleJp: '心配をかけてごめん。', exampleEn: '"Sorry for causing worry."' },
  { word: '生活', reading: 'せいかつ', meaning: 'daily life; living', type: 'noun', exampleJp: '生活が豊かになった。', exampleEn: '"My life became rich."' },
  { word: '説明', reading: 'せつめい', meaning: 'explanation', type: 'noun', exampleJp: '説明を聞く。', exampleEn: '"I listen to the explanation."' },
  { word: '相談', reading: 'そうだん', meaning: 'consultation; discussion', type: 'noun', exampleJp: '先生に相談する。', exampleEn: '"I consult with the teacher."' },
  { word: '注意', reading: 'ちゅうい', meaning: 'caution; attention', type: 'noun', exampleJp: '注意して運転する。', exampleEn: '"I drive carefully."' },
  { word: '日常', reading: 'にちじょう', meaning: 'everyday; daily life', type: 'noun', exampleJp: '日常の生活を大切にする。', exampleEn: '"I value everyday life."' },
  { word: '費用', reading: 'ひよう', meaning: 'cost; expense', type: 'noun', exampleJp: '費用を計算する。', exampleEn: '"I calculate the cost."' },
  { word: '文化', reading: 'ぶんか', meaning: 'culture', type: 'noun', exampleJp: '日本の文化を学ぶ。', exampleEn: '"I learn about Japanese culture."' },
  { word: '方法', reading: 'ほうほう', meaning: 'method; way', type: 'noun', exampleJp: '勉強の方法を変える。', exampleEn: '"I change my study method."' },
  { word: '理由', reading: 'りゆう', meaning: 'reason', type: 'noun', exampleJp: '理由を説明する。', exampleEn: '"I explain the reason."' },
  { word: '旅行', reading: 'りょこう', meaning: 'travel; trip', type: 'noun', exampleJp: '旅行の計画を立てる。', exampleEn: '"I make travel plans."' },
  { word: '明るい', reading: 'あかるい', meaning: 'bright; cheerful', type: 'i-adjective', exampleJp: '部屋が明るい。', exampleEn: '"The room is bright."' },
  { word: '怪しい', reading: 'あやしい', meaning: 'suspicious; strange', type: 'i-adjective', exampleJp: '怪しい人がいる。', exampleEn: '"There is a suspicious person."' },
  { word: '忙しい', reading: 'いそがしい', meaning: 'busy', type: 'i-adjective', exampleJp: '毎日忙しい。', exampleEn: '"I am busy every day."' },
  { word: '嬉しい', reading: 'うれしい', meaning: 'happy; glad', type: 'i-adjective', exampleJp: '合格して嬉しい。', exampleEn: '"I am happy to have passed."' },
  { word: '悲しい', reading: 'かなしい', meaning: 'sad', type: 'i-adjective', exampleJp: '別れが悲しい。', exampleEn: '"The farewell is sad."' },
  { word: '難しい', reading: 'むずかしい', meaning: 'difficult', type: 'i-adjective', exampleJp: 'この問題は難しい。', exampleEn: '"This problem is difficult."' },
  { word: '珍しい', reading: 'めずらしい', meaning: 'rare; unusual', type: 'i-adjective', exampleJp: '珍しい鳥を見た。', exampleEn: '"I saw a rare bird."' },
  { word: '厳しい', reading: 'きびしい', meaning: 'strict; severe', type: 'i-adjective', exampleJp: '先生が厳しい。', exampleEn: '"The teacher is strict."' },
  { word: '悔しい', reading: 'くやしい', meaning: 'frustrated; vexed', type: 'i-adjective', exampleJp: '負けて悔しい。', exampleEn: '"I feel frustrated after losing."' },
  { word: '正しい', reading: 'ただしい', meaning: 'correct; right', type: 'i-adjective', exampleJp: '正しい答えを選ぶ。', exampleEn: '"I choose the correct answer."' },
  { word: '便利', reading: 'べんり', meaning: 'convenient', type: 'na-adjective', exampleJp: 'スマホは便利だ。', exampleEn: '"Smartphones are convenient."' },
  { word: '不便', reading: 'ふべん', meaning: 'inconvenient', type: 'na-adjective', exampleJp: '田舎は不便だ。', exampleEn: '"The countryside is inconvenient."' },
  { word: '正確', reading: 'せいかく', meaning: 'accurate; precise', type: 'na-adjective', exampleJp: '正確な情報が必要だ。', exampleEn: '"Accurate information is needed."' },
  { word: '大切', reading: 'たいせつ', meaning: 'important; precious', type: 'na-adjective', exampleJp: '時間を大切にする。', exampleEn: '"I value my time."' },
  { word: '必要', reading: 'ひつよう', meaning: 'necessary', type: 'na-adjective', exampleJp: '努力が必要だ。', exampleEn: '"Effort is necessary."' },
  { word: '豊か', reading: 'ゆたか', meaning: 'rich; abundant', type: 'na-adjective', exampleJp: '自然が豊かな国だ。', exampleEn: '"It is a country rich in nature."' },
  { word: '適切', reading: 'てきせつ', meaning: 'appropriate; suitable', type: 'na-adjective', exampleJp: '適切な言葉を選ぶ。', exampleEn: '"I choose appropriate words."' },
  { word: '十分', reading: 'じゅうぶん', meaning: 'sufficient; enough', type: 'na-adjective', exampleJp: '準備は十分だ。', exampleEn: '"The preparation is sufficient."' },
  { word: '真剣', reading: 'しんけん', meaning: 'serious; earnest', type: 'na-adjective', exampleJp: '真剣に考える。', exampleEn: '"I think seriously."' },
  { word: '安全', reading: 'あんぜん', meaning: 'safe; secure', type: 'na-adjective', exampleJp: 'この道は安全だ。', exampleEn: '"This road is safe."' },
  { word: '特別', reading: 'とくべつ', meaning: 'special', type: 'na-adjective', exampleJp: '今日は特別な日だ。', exampleEn: '"Today is a special day."' },
];

const DEFAULT_ROUND_SIZE = 10;

function shuffle(arr) {
  const a = [...arr];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

function timeAgo(date) {
  const sec = Math.floor((Date.now() - date) / 1000);
  const min = Math.floor(sec / 60);
  if (min < 1) return 'just now';
  if (min < 60) return min + ' minute' + (min === 1 ? '' : 's') + ' ago';
  const hr = Math.floor(min / 60);
  if (hr < 24) return hr + ' hour' + (hr === 1 ? '' : 's') + ' ago';
  const day = Math.floor(hr / 24);
  return day + ' day' + (day === 1 ? '' : 's') + ' ago';
}

// Session state
let poolSize = words.length;
let roundSize = DEFAULT_ROUND_SIZE;
let pool = shuffle([...words]);
let round = 1;
let redo = [];
let doneCount = 0;
let drillStartedAt = Date.now();

function buildRound() {
  const slots = Math.max(0, roundSize - redo.length);
  const picked = pool.splice(0, slots);
  return [...redo, ...picked];
}

let remaining = buildRound();
let currentWord = remaining[0];

function updateStats() {
  document.getElementById('stat-togo').textContent = (poolSize - doneCount) + ' to go of ' + poolSize;
  document.getElementById('sidebar-title').textContent = 'Round ' + round;
  document.getElementById('header-began').textContent = 'began ' + timeAgo(drillStartedAt);

  const pct = (doneCount / poolSize) * 100;
  document.querySelector('.progress-bar').style.width = pct + '%';
}

function showWord() {
  document.getElementById('prompt-word-jp').textContent = currentWord.word;
  document.getElementById('prompt-example-jp').textContent = currentWord.exampleJp;

  const list = document.getElementById('sidebar-list');
  list.querySelectorAll('.sidebar-item.current').forEach(el => el.classList.remove('current'));
  const item = list.querySelector('[data-id="' + currentWord.word + '"]');
  if (item) item.classList.add('current');
}

function reveal(knew) {
  const answered = currentWord;

  // Update drill state
  remaining.shift();
  if (knew) {
    doneCount++;
  } else {
    redo.push(answered);
  }

  addToSidebar(answered, knew);
  updateStats();

  // Show answered word in last-word card
  document.getElementById('last-word-card').style.display = '';
  const lastWordEl = document.getElementById('last-word-jp');
  lastWordEl.textContent = answered.word;
  lastWordEl.className = 'tooltip-word ' + (knew ? 'knew' : 'missed');
  document.getElementById('last-reading').textContent = answered.reading;
  document.getElementById('last-pos').textContent = answered.type;
  document.getElementById('last-meaning').textContent = answered.meaning;
  document.getElementById('last-example-jp').textContent = answered.exampleJp;
  document.getElementById('last-example-en').textContent = answered.exampleEn;

  // Advance
  if (remaining.length === 0) {
    if (redo.length > 0 || pool.length > 0) {
      startNextRound();
      return;
    } else {
      document.getElementById('prompt-word-jp').textContent = 'Done!';
      document.getElementById('prompt-example-jp').textContent = 'All words cleared.';
      document.getElementById('action-prompt').style.display = 'none';
      return;
    }
  }

  currentWord = remaining[0];
  showWord();
}

function initSidebar() {
  const list = document.getElementById('sidebar-list');
  remaining.forEach(word => {
    const li = document.createElement('li');
    li.className = 'sidebar-item unseen';
    li.textContent = word.word;
    li.dataset.word = JSON.stringify(word);
    li.dataset.id = word.word;
    list.appendChild(li);
  });
}
initSidebar();

function addToSidebar(word, knew) {
  const list = document.getElementById('sidebar-list');

  // Remove existing entry for this word if present
  const existing = list.querySelector('[data-id="' + word.word + '"]');
  if (existing) existing.remove();

  const li = document.createElement('li');
  li.className = 'sidebar-item ' + (knew ? 'known flash-known' : 'missed flash-missed');
  li.textContent = word.word;
  li.dataset.word = JSON.stringify(word);
  li.dataset.id = word.word;
  li.addEventListener('animationend', () => li.classList.remove('flash-known', 'flash-missed'));

  // Order: missed, then known, then unseen-redo, then unseen
  if (!knew) {
    // Missed: before known, unseen-redo, and unseen
    const firstNonMissed = list.querySelector('.sidebar-item.known, .sidebar-item.unseen-redo, .sidebar-item.unseen');
    if (firstNonMissed) {
      list.insertBefore(li, firstNonMissed);
    } else {
      list.appendChild(li);
    }
  } else {
    // Known: before unseen-redo and unseen
    const firstUnseen = list.querySelector('.sidebar-item.unseen-redo, .sidebar-item.unseen');
    if (firstUnseen) {
      list.insertBefore(li, firstUnseen);
    } else {
      list.appendChild(li);
    }
  }
}

// Tooltip hover logic
const tip = document.getElementById('tooltip');
document.getElementById('sidebar-list').addEventListener('mouseover', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item || !item.dataset.word) return;
  const data = JSON.parse(item.dataset.word);
  document.getElementById('tip-word').textContent = data.word;
  document.getElementById('tip-reading').textContent = data.reading;
  document.getElementById('tip-pos').textContent = data.type;
  document.getElementById('tip-meaning').textContent = data.meaning;
  document.getElementById('tip-example').textContent = data.exampleJp || '';
  document.getElementById('tip-example-en').textContent = data.exampleEn || '';

  const rect = item.getBoundingClientRect();
  const sidebar = document.querySelector('.sidebar');
  tip.style.left = sidebar.getBoundingClientRect().right + 'px';
  tip.style.top = rect.top + 'px';
  tip.style.transform = '';
  tip.classList.add('visible');
});
document.getElementById('sidebar-list').addEventListener('mouseout', e => {
  const item = e.target.closest('.sidebar-item');
  if (!item) return;
  if (!item.contains(e.relatedTarget)) {
    tip.classList.remove('visible');
  }
});


function startNextRound() {
  round++;
  const redoSet = new Set(redo.map(w => w.word));
  remaining = buildRound(); // uses current redo + new picks from pool
  redo = [];
  currentWord = remaining[0];
  updateStats();

  const list = document.getElementById('sidebar-list');
  list.innerHTML = '';

  // Redo words first (red + blurred), then new words (gray + blurred)
  const redoWords = remaining.filter(w => redoSet.has(w.word));
  const newWords = remaining.filter(w => !redoSet.has(w.word));
  [...redoWords, ...newWords].forEach(word => {
    const isRedo = redoSet.has(word.word);
    const li = document.createElement('li');
    li.className = 'sidebar-item ' + (isRedo ? 'unseen-redo' : 'unseen');
    li.textContent = word.word;
    li.dataset.word = JSON.stringify(word);
    li.dataset.id = word.word;
    list.appendChild(li);
  });

  document.getElementById('last-word-card').style.display = 'none';
  showWord();
}

const STEP_INTERVAL = 230;
let _stepTimer = null;
function startStep(fn, ...args) { fn(...args); _stepTimer = setInterval(() => fn(...args), STEP_INTERVAL); }
function stopStep() { clearInterval(_stepTimer); _stepTimer = null; }

function adjustRestart(id, delta) {
  const input = document.getElementById(id);
  const val = parseInt(input.value, 10) || 5;
  input.value = delta > 0
    ? Math.min(995, Math.floor(val / 5) * 5 + 5)
    : Math.max(5, Math.ceil(val / 5) * 5 - 5);
}

function capRestartInput(input) {
  if (input.value.length > 3) input.value = input.value.slice(0, 3);
  if (input.value === '0') input.value = '1';
}

function openRestartModal() {
  document.getElementById('restart-total-words').value = poolSize;
  document.getElementById('restart-round-size').value = roundSize;
  document.getElementById('restart-modal-backdrop').classList.remove('hidden');
}
function closeRestartModal() {
  document.getElementById('restart-modal-backdrop').classList.add('hidden');
}
function handleRestartBackdropClick(e) {
  if (e.target === document.getElementById('restart-modal-backdrop')) closeRestartModal();
}
function confirmRestart() {
  const total = Math.max(1, Math.min(parseInt(document.getElementById('restart-total-words').value, 10) || poolSize, words.length));
  const rSize = Math.max(1, parseInt(document.getElementById('restart-round-size').value, 10) || roundSize);
  closeRestartModal();
  restartDrill(total, rSize);
}

function restartDrill(totalWords, newRoundSize) {
  poolSize = totalWords;
  roundSize = newRoundSize;
  pool = shuffle([...words]).slice(0, poolSize);
  round = 1;
  redo = [];
  doneCount = 0;
  drillStartedAt = Date.now();
  remaining = buildRound();
  currentWord = remaining[0];

  document.getElementById('sidebar-list').innerHTML = '';
  document.getElementById('action-prompt').style.display = '';
  document.getElementById('last-word-card').style.display = 'none';
  initSidebar();
  updateStats();
  showWord();
}

// Initialize
showWord();
updateStats();

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') { closeRestartModal(); return; }
  const prompt = document.getElementById('action-prompt');
  if (prompt.style.display === 'none') return;
  if (e.key === 'd' || e.key === 'D') reveal(true);
  if (e.key === 'a' || e.key === 'A') reveal(false);
});

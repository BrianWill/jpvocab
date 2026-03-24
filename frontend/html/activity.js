const TODAY = '2026-03-23';
const HISTORY_START = '2025-11-02'; // Sunday of oldest week with history

// Word dictionary: word -> [reading, meaning]
const W = {
  '走る':   ['はしる',   'to run'],
  '話す':   ['はなす',   'to speak'],
  '始める': ['はじめる', 'to begin'],
  '忘れる': ['わすれる', 'to forget'],
  '選ぶ':   ['えらぶ',   'to choose'],
  '集める': ['あつめる', 'to collect'],
  '届く':   ['とどく',   'to reach; to arrive'],
  '伝える': ['つたえる', 'to convey; to tell'],
  '借りる': ['かりる',   'to borrow'],
  '探す':   ['さがす',   'to search for'],
  '壊れる': ['こわれる', 'to break; to be broken'],
  '誘う':   ['さそう',   'to invite'],
  '受ける': ['うける',   'to receive; to take'],
  '捨てる': ['すてる',   'to throw away'],
  '泳ぐ':   ['およぐ',   'to swim'],
  '運ぶ':   ['はこぶ',   'to carry; to transport'],
  '起きる': ['おきる',   'to wake up; to occur'],
  '落ちる': ['おちる',   'to fall; to drop'],
  '変える': ['かえる',   'to change'],
  '考える': ['かんがえる','to think; to consider'],
  '決める': ['きめる',   'to decide'],
  '困る':   ['こまる',   'to be troubled'],
  '調べる': ['しらべる', 'to investigate; to look up'],
  '信じる': ['しんじる', 'to believe'],
  '続ける': ['つづける', 'to continue'],
  '手伝う': ['てつだう', 'to help; to assist'],
  '直す':   ['なおす',   'to fix; to correct'],
  '並ぶ':   ['ならぶ',   'to line up'],
  '逃げる': ['にげる',   'to run away; to escape'],
  '脱ぐ':   ['ぬぐ',     'to take off (clothes)'],
  '登る':   ['のぼる',   'to climb'],
  '払う':   ['はらう',   'to pay'],
  '減る':   ['へる',     'to decrease'],
  '曲がる': ['まがる',   'to turn; to bend'],
  '守る':   ['まもる',   'to protect; to keep'],
  '見える': ['みえる',   'to be visible; to appear'],
  '向かう': ['むかう',   'to head towards'],
  '戻る':   ['もどる',   'to return'],
  '焼く':   ['やく',     'to grill; to bake'],
  '触る':   ['さわる',   'to touch'],
  '静か':   ['しずか',   'quiet; calm'],
  '暖かい': ['あたたかい','warm'],
  '危ない': ['あぶない', 'dangerous'],
  '眠い':   ['ねむい',   'sleepy'],
  '丁寧':   ['ていねい', 'polite; careful'],
  '複雑':   ['ふくざつ', 'complicated'],
  '残念':   ['ざんねん', 'unfortunate; regrettable'],
  '優しい': ['やさしい', 'kind; gentle'],
  '自由':   ['じゆう',   'freedom; free'],
  '明るい': ['あかるい', 'bright; cheerful'],
  '忙しい': ['いそがしい','busy'],
  '嬉しい': ['うれしい', 'happy; glad'],
  '悲しい': ['かなしい', 'sad'],
  '難しい': ['むずかしい','difficult'],
  '正しい': ['ただしい', 'correct; right'],
  '便利':   ['べんり',   'convenient'],
  '大切':   ['たいせつ', 'important; precious'],
  '必要':   ['ひつよう', 'necessary'],
  '安全':   ['あんぜん', 'safe; safety'],
  '特別':   ['とくべつ', 'special'],
  '珍しい': ['めずらしい','rare; unusual'],
  '厳しい': ['きびしい', 'strict; harsh'],
  '悔しい': ['くやしい', 'frustrating; vexing'],
  '怪しい': ['あやしい', 'suspicious; shady'],
  '豊か':   ['ゆたか',   'abundant; rich'],
  '適切':   ['てきせつ', 'appropriate; suitable'],
  '十分':   ['じゅうぶん','enough; sufficient'],
  '真剣':   ['しんけん', 'serious; earnest'],
  '正確':   ['せいかく', 'accurate; precise'],
  '不便':   ['ふべん',   'inconvenient'],
  '天気':   ['てんき',   'weather'],
  '経験':   ['けいけん', 'experience'],
  '練習':   ['れんしゅう','practice'],
  '景色':   ['けしき',   'scenery; view'],
  '約束':   ['やくそく', 'promise'],
  '意見':   ['いけん',   'opinion'],
  '計画':   ['けいかく', 'plan'],
  '準備':   ['じゅんび', 'preparation'],
  '心配':   ['しんぱい', 'worry; concern'],
  '問題':   ['もんだい', 'problem; question'],
  '機会':   ['きかい',   'opportunity; chance'],
  '記念':   ['きねん',   'commemoration; memory'],
  '原因':   ['げんいん', 'cause; origin'],
  '事故':   ['じこ',     'accident'],
  '場所':   ['ばしょ',   'place; location'],
  '生活':   ['せいかつ', 'daily life; living'],
  '説明':   ['せつめい', 'explanation'],
  '相談':   ['そうだん', 'consultation; discussion'],
  '注意':   ['ちゅうい', 'caution; attention'],
  '費用':   ['ひよう',   'cost; expense'],
  '文化':   ['ぶんか',   'culture'],
  '方法':   ['ほうほう', 'method; way'],
  '理由':   ['りゆう',   'reason'],
  '旅行':   ['りょこう', 'travel; trip'],
  '日常':   ['にちじょう','everyday; daily routine'],
};

// dr(word, knew) — drill entry
function dr(word, knew) {
  return { word, reading: W[word][0], meaning: W[word][1], knew };
}

// wr(word) — word-only entry (for added/cleared)
function wr(word) {
  return { word, reading: W[word][0], meaning: W[word][1] };
}

const activityData = {
  // ── Week of Mar 23 (current week) ──────────────────────────────────────────
  '2026-03-23': {
    drilled: [
      dr('旅行', true), dr('景色', true), dr('経験', false),
      dr('文化', true), dr('記念', false),
    ],
    added: [],
    cleared: [],
  },

  // ── Week of Mar 16 ─────────────────────────────────────────────────────────
  '2026-03-16': {
    drilled: [
      dr('走る', true), dr('話す', true), dr('始める', false),
      dr('忘れる', true), dr('選ぶ', true), dr('集める', false),
      dr('届く', true), dr('伝える', true),
    ],
    added: [wr('旅行'), wr('景色'), wr('経験'), wr('文化'), wr('記念')],
    cleared: [],
  },
  '2026-03-17': {
    drilled: [
      dr('借りる', true), dr('探す', false), dr('壊れる', false),
      dr('誘う', true), dr('受ける', true), dr('捨てる', true),
      dr('泳ぐ', true), dr('運ぶ', false), dr('起きる', true),
      dr('落ちる', true),
    ],
    added: [],
    cleared: [],
  },
  '2026-03-18': {
    drilled: [
      dr('変える', true), dr('考える', true), dr('決める', false),
      dr('困る', true), dr('調べる', true), dr('信じる', true),
      dr('続ける', false),
    ],
    added: [wr('相談'), wr('説明')],
    cleared: [],
  },
  '2026-03-19': {
    drilled: [
      dr('手伝う', true), dr('直す', true), dr('並ぶ', true),
      dr('逃げる', false), dr('脱ぐ', true), dr('登る', true),
      dr('払う', true), dr('減る', false), dr('曲がる', true),
      dr('守る', true),
    ],
    added: [],
    cleared: [wr('走る'), wr('話す'), wr('選ぶ')],
  },
  // Mar 20 (Fri) — rest day, no entry
  '2026-03-21': {
    drilled: [
      dr('見える', true), dr('向かう', false), dr('戻る', true),
      dr('焼く', true), dr('触る', true), dr('静か', true),
    ],
    added: [],
    cleared: [],
  },
  // Mar 22 (Sun) — rest day, no entry

  // ── Week of Mar 9 ──────────────────────────────────────────────────────────
  '2026-03-09': {
    drilled: [
      dr('暖かい', true), dr('危ない', false), dr('眠い', true),
      dr('丁寧', true), dr('複雑', false), dr('残念', true),
      dr('優しい', true), dr('自由', true),
    ],
    added: [wr('見える'), wr('向かう'), wr('戻る'), wr('焼く'), wr('触る')],
    cleared: [],
  },
  '2026-03-10': {
    drilled: [
      dr('明るい', true), dr('忙しい', true), dr('嬉しい', false),
      dr('悲しい', true), dr('難しい', false), dr('正しい', true),
      dr('便利', true), dr('大切', true), dr('必要', false),
      dr('安全', true),
    ],
    added: [],
    cleared: [],
  },
  '2026-03-11': {
    drilled: [
      dr('特別', true), dr('珍しい', false), dr('厳しい', true),
      dr('悔しい', true), dr('怪しい', false), dr('豊か', true),
      dr('適切', true), dr('十分', true),
    ],
    added: [wr('天気'), wr('練習'), wr('準備')],
    cleared: [wr('借りる'), wr('泳ぐ')],
  },
  // Mar 12 (Thu) — rest day
  '2026-03-13': {
    drilled: [
      dr('真剣', true), dr('正確', false), dr('不便', true),
      dr('天気', true), dr('経験', true), dr('練習', true),
      dr('約束', false), dr('意見', true), dr('計画', true),
      dr('準備', true),
    ],
    added: [],
    cleared: [],
  },
  '2026-03-14': {
    drilled: [
      dr('心配', false), dr('問題', true), dr('機会', true),
      dr('記念', true), dr('原因', false), dr('事故', true),
    ],
    added: [wr('約束'), wr('意見'), wr('計画')],
    cleared: [],
  },
  // Mar 15 (Sun) — rest day

  // ── Week of Mar 2 ──────────────────────────────────────────────────────────
  '2026-03-02': {
    drilled: [
      dr('場所', true), dr('生活', true), dr('説明', false),
      dr('相談', true), dr('注意', true), dr('費用', false),
      dr('文化', true), dr('方法', true),
    ],
    added: [wr('場所'), wr('生活'), wr('注意'), wr('費用'), wr('方法')],
    cleared: [],
  },
  '2026-03-03': {
    drilled: [
      dr('理由', true), dr('日常', true), dr('景色', false),
      dr('旅行', true), dr('経験', true), dr('文化', true),
      dr('記念', false), dr('問題', true), dr('原因', true),
      dr('事故', false),
    ],
    added: [],
    cleared: [],
  },
  '2026-03-04': {
    drilled: [
      dr('走る', true), dr('忘れる', true), dr('集める', false),
      dr('届く', true), dr('壊れる', true), dr('受ける', false),
      dr('捨てる', true), dr('運ぶ', true),
    ],
    added: [wr('理由'), wr('日常')],
    cleared: [wr('届く'), wr('運ぶ'), wr('受ける')],
  },
  // Mar 5 (Thu) — rest day
  '2026-03-06': {
    drilled: [
      dr('考える', true), dr('決める', true), dr('信じる', false),
      dr('続ける', true), dr('逃げる', true), dr('払う', false),
      dr('守る', true), dr('静か', true),
    ],
    added: [],
    cleared: [],
  },
  '2026-03-07': {
    drilled: [
      dr('暖かい', true), dr('丁寧', true), dr('優しい', false),
      dr('自由', true), dr('明るい', true), dr('忙しい', false),
      dr('難しい', true), dr('大切', true), dr('必要', true),
      dr('安全', false),
    ],
    added: [],
    cleared: [wr('考える'), wr('続ける'), wr('守る')],
  },
  // Mar 8 (Sun) — rest day

  // ── Week of Feb 23 (Feb 23 – Mar 1) ────────────────────────────────────────
  '2026-02-23': {
    drilled: [
      dr('話す', true), dr('始める', false), dr('選ぶ', true),
      dr('伝える', true), dr('借りる', false), dr('探す', true),
      dr('誘う', true), dr('起きる', true),
    ],
    added: [wr('暖かい'), wr('危ない'), wr('眠い'), wr('丁寧'), wr('複雑'), wr('残念')],
    cleared: [],
  },
  '2026-02-24': {
    drilled: [
      dr('落ちる', true), dr('変える', false), dr('困る', true),
      dr('調べる', true), dr('手伝う', true), dr('直す', false),
      dr('並ぶ', true), dr('脱ぐ', true),
    ],
    added: [],
    cleared: [],
  },
  '2026-02-25': {
    drilled: [
      dr('登る', true), dr('減る', false), dr('曲がる', true),
      dr('見える', true), dr('焼く', true), dr('触る', false),
      dr('複雑', true), dr('残念', true), dr('優しい', true),
      dr('豊か', false),
    ],
    added: [wr('場所'), wr('生活'), wr('説明'), wr('相談')],
    cleared: [wr('話す'), wr('伝える'), wr('誘う')],
  },
  // Feb 26 (Thu) — rest day
  '2026-02-27': {
    drilled: [
      dr('特別', true), dr('珍しい', true), dr('厳しい', false),
      dr('悔しい', true), dr('怪しい', true), dr('真剣', false),
      dr('正確', true), dr('不便', true),
    ],
    added: [],
    cleared: [],
  },
  '2026-02-28': {
    drilled: [
      dr('天気', true), dr('練習', false), dr('景色', true),
      dr('約束', true), dr('意見', false), dr('計画', true),
      dr('準備', true), dr('心配', true), dr('問題', false),
      dr('機会', true),
    ],
    added: [wr('走る'), wr('話す'), wr('始める'), wr('忘れる'), wr('選ぶ'), wr('集める')],
    cleared: [],
  },
  '2026-03-01': {
    drilled: [
      dr('注意', true), dr('費用', true), dr('方法', false),
      dr('理由', true), dr('日常', true), dr('安全', true),
      dr('便利', false), dr('正しい', true),
    ],
    added: [],
    cleared: [wr('特別'), wr('真剣'), wr('正確')],
  },

  // ── February 2026 (early) ─────────────────────────────────────────────────
  '2026-02-02': {
    drilled: [dr('考える',true), dr('決める',true), dr('困る',false), dr('続ける',true), dr('払う',true), dr('守る',false), dr('向かう',true)],
    added: [], cleared: [],
  },
  '2026-02-04': {
    drilled: [dr('逃げる',true), dr('直す',false), dr('見える',true), dr('戻る',true), dr('焼く',false), dr('静か',true), dr('豊か',true)],
    added: [], cleared: [],
  },
  '2026-02-06': {
    drilled: [dr('難しい',true), dr('必要',true), dr('大切',false), dr('安全',true), dr('自由',true), dr('明るい',false), dr('忙しい',true)],
    added: [], cleared: [],
  },
  '2026-02-09': {
    drilled: [dr('特別',true), dr('珍しい',false), dr('厳しい',true), dr('真剣',true), dr('正確',false), dr('不便',true), dr('悔しい',true)],
    added: [], cleared: [wr('考える'), wr('決める'), wr('守る')],
  },
  '2026-02-11': {
    drilled: [dr('天気',true), dr('経験',false), dr('練習',true), dr('準備',true), dr('心配',false), dr('問題',true), dr('機会',true)],
    added: [], cleared: [],
  },
  '2026-02-13': {
    drilled: [dr('記念',true), dr('原因',false), dr('事故',true), dr('景色',true), dr('旅行',false), dr('文化',true), dr('説明',true)],
    added: [], cleared: [],
  },
  '2026-02-16': {
    drilled: [dr('相談',true), dr('日常',false), dr('理由',true), dr('困る',true), dr('続ける',true), dr('払う',false), dr('便利',true)],
    added: [], cleared: [],
  },
  '2026-02-18': {
    drilled: [dr('信じる',true), dr('調べる',false), dr('手伝う',true), dr('並ぶ',true), dr('逃げる',false), dr('脱ぐ',true), dr('登る',true)],
    added: [], cleared: [wr('信じる'), wr('練習'), wr('準備')],
  },
  '2026-02-20': {
    drilled: [dr('減る',true), dr('曲がる',false), dr('見える',true), dr('戻る',true), dr('焼く',true), dr('触る',false), dr('静か',true), dr('豊か',true)],
    added: [], cleared: [],
  },

  // ── January 2026 ──────────────────────────────────────────────────────────
  '2026-01-02': {
    drilled: [dr('考える',true), dr('決める',false), dr('困る',true), dr('直す',true), dr('守る',true), dr('景色',false), dr('旅行',true)],
    added: [], cleared: [],
  },
  '2026-01-05': {
    drilled: [dr('逃げる',true), dr('払う',true), dr('続ける',false), dr('信じる',true), dr('調べる',true), dr('日常',false), dr('理由',true)],
    added: [], cleared: [],
  },
  '2026-01-07': {
    drilled: [dr('手伝う',true), dr('並ぶ',false), dr('脱ぐ',true), dr('登る',true), dr('減る',false), dr('曲がる',true), dr('豊か',true)],
    added: [], cleared: [wr('難しい'), wr('大切'), wr('安全')],
  },
  '2026-01-09': {
    drilled: [dr('見える',true), dr('向かう',true), dr('戻る',false), dr('焼く',true), dr('触る',true), dr('静か',false), dr('自由',true), dr('明るい',true)],
    added: [], cleared: [],
  },
  '2026-01-12': {
    drilled: [dr('忙しい',true), dr('必要',false), dr('便利',true), dr('正しい',true), dr('特別',false), dr('珍しい',true), dr('厳しい',true)],
    added: [], cleared: [],
  },
  '2026-01-14': {
    drilled: [dr('悔しい',true), dr('怪しい',false), dr('真剣',true), dr('正確',true), dr('不便',false), dr('天気',true), dr('経験',true)],
    added: [], cleared: [],
  },
  '2026-01-16': {
    drilled: [dr('練習',true), dr('準備',false), dr('心配',true), dr('問題',true), dr('機会',false), dr('記念',true), dr('原因',true)],
    added: [], cleared: [wr('明るい'), wr('忙しい'), wr('便利')],
  },
  '2026-01-19': {
    drilled: [dr('事故',true), dr('景色',false), dr('旅行',true), dr('文化',true), dr('説明',false), dr('相談',true), dr('日常',true)],
    added: [], cleared: [],
  },
  '2026-01-21': {
    drilled: [dr('理由',true), dr('考える',false), dr('決める',true), dr('困る',true), dr('直す',false), dr('守る',true), dr('向かう',true)],
    added: [], cleared: [],
  },
  '2026-01-23': {
    drilled: [dr('逃げる',true), dr('払う',false), dr('続ける',true), dr('信じる',true), dr('調べる',false), dr('手伝う',true), dr('並ぶ',true)],
    added: [], cleared: [wr('特別'), wr('怪しい'), wr('正確')],
  },
  '2026-01-26': {
    drilled: [dr('脱ぐ',true), dr('登る',true), dr('減る',false), dr('曲がる',true), dr('見える',true), dr('戻る',false), dr('焼く',true)],
    added: [], cleared: [],
  },
  '2026-01-28': {
    drilled: [dr('触る',true), dr('静か',true), dr('自由',false), dr('豊か',true), dr('天気',true), dr('練習',false), dr('準備',true), dr('問題',true)],
    added: [], cleared: [],
  },
  '2026-01-30': {
    drilled: [dr('心配',true), dr('機会',false), dr('記念',true), dr('原因',true), dr('事故',false), dr('説明',true), dr('相談',true)],
    added: [], cleared: [wr('正しい'), wr('不便')],
  },

  // ── December 2025 ─────────────────────────────────────────────────────────
  '2025-12-01': {
    drilled: [dr('困る',false), dr('逃げる',true), dr('払う',true), dr('続ける',true), dr('信じる',false), dr('見える',true), dr('向かう',true)],
    added: [wr('困る'), wr('逃げる'), wr('払う'), wr('続ける'), wr('信じる')],
    cleared: [],
  },
  '2025-12-03': {
    drilled: [dr('困る',true), dr('逃げる',true), dr('払う',false), dr('続ける',true), dr('信じる',true), dr('守る',true), dr('考える',false)],
    added: [], cleared: [],
  },
  '2025-12-05': {
    drilled: [dr('手伝う',true), dr('並ぶ',false), dr('脱ぐ',true), dr('登る',true), dr('減る',false), dr('曲がる',true)],
    added: [wr('手伝う'), wr('並ぶ'), wr('脱ぐ'), wr('登る'), wr('減る'), wr('曲がる')],
    cleared: [wr('静か'), wr('自由')],
  },
  '2025-12-08': {
    drilled: [dr('手伝う',true), dr('並ぶ',true), dr('脱ぐ',false), dr('登る',true), dr('困る',true), dr('逃げる',false), dr('払う',true)],
    added: [], cleared: [],
  },
  '2025-12-10': {
    drilled: [dr('豊か',true), dr('日常',false), dr('理由',true), dr('続ける',true), dr('信じる',true), dr('手伝う',true), dr('脱ぐ',false)],
    added: [wr('豊か'), wr('日常'), wr('理由')],
    cleared: [],
  },
  '2025-12-12': {
    drilled: [dr('豊か',true), dr('日常',true), dr('理由',false), dr('登る',true), dr('減る',true), dr('曲がる',false), dr('景色',true), dr('旅行',true)],
    added: [], cleared: [wr('難しい'), wr('大切')],
  },
  '2025-12-15': {
    drilled: [dr('逃げる',true), dr('払う',true), dr('続ける',true), dr('信じる',false), dr('文化',true), dr('説明',false), dr('相談',true), dr('豊か',true)],
    added: [], cleared: [],
  },
  '2025-12-17': {
    drilled: [dr('困る',true), dr('調べる',true), dr('手伝う',false), dr('並ぶ',true), dr('日常',true), dr('理由',true), dr('天気',true), dr('経験',false)],
    added: [wr('調べる')], cleared: [],
  },
  '2025-12-19': {
    drilled: [dr('脱ぐ',true), dr('登る',true), dr('減る',false), dr('曲がる',true), dr('特別',true), dr('真剣',true), dr('正確',false), dr('悔しい',true)],
    added: [], cleared: [wr('必要'), wr('便利')],
  },
  '2025-12-22': {
    drilled: [dr('豊か',true), dr('日常',true), dr('理由',true), dr('景色',false), dr('旅行',true), dr('文化',true), dr('練習',true), dr('問題',true)],
    added: [], cleared: [],
  },
  '2025-12-26': {
    drilled: [dr('心配',true), dr('機会',false), dr('記念',true), dr('原因',true), dr('説明',true), dr('相談',false), dr('珍しい',true), dr('厳しい',true)],
    added: [], cleared: [],
  },
  '2025-12-29': {
    drilled: [dr('逃げる',true), dr('払う',true), dr('続ける',false), dr('信じる',true), dr('調べる',true), dr('手伝う',true), dr('並ぶ',false), dr('豊か',true)],
    added: [], cleared: [wr('景色'), wr('旅行')],
  },

  // ── November 2025 ─────────────────────────────────────────────────────────
  '2025-11-03': {
    drilled: [dr('考える',true), dr('決める',false), dr('困る',true), dr('直す',true), dr('守る',false), dr('見える',true)],
    added: [wr('考える'), wr('決める'), wr('困る'), wr('直す'), wr('守る'), wr('見える'), wr('向かう'), wr('戻る')],
    cleared: [],
  },
  '2025-11-05': {
    drilled: [dr('考える',true), dr('決める',true), dr('困る',false), dr('直す',true), dr('守る',true), dr('見える',false), dr('向かう',true), dr('戻る',true)],
    added: [wr('焼く'), wr('触る'), wr('静か'), wr('自由'), wr('明るい'), wr('忙しい')],
    cleared: [],
  },
  '2025-11-07': {
    drilled: [dr('焼く',false), dr('触る',true), dr('静か',true), dr('自由',true), dr('明るい',false), dr('忙しい',true), dr('考える',true), dr('守る',true)],
    added: [wr('難しい'), wr('大切'), wr('必要'), wr('安全'), wr('便利'), wr('正しい')],
    cleared: [],
  },
  '2025-11-10': {
    drilled: [dr('難しい',true), dr('大切',false), dr('必要',true), dr('安全',true), dr('便利',false), dr('正しい',true), dr('静か',true), dr('自由',true)],
    added: [wr('特別'), wr('珍しい'), wr('厳しい')],
    cleared: [],
  },
  '2025-11-12': {
    drilled: [dr('特別',false), dr('珍しい',true), dr('厳しい',false), dr('難しい',true), dr('必要',true), dr('安全',true), dr('明るい',true)],
    added: [wr('悔しい'), wr('怪しい'), wr('真剣'), wr('正確')],
    cleared: [],
  },
  '2025-11-14': {
    drilled: [
      dr('悔しい',true), dr('怪しい',false), dr('真剣',true), dr('正確',true), dr('特別',true), dr('珍しい',true), dr('厳しい',true), dr('大切',true),
      dr('難しい',false), dr('必要',true), dr('安全',true), dr('便利',false), dr('正しい',true), dr('自由',true), dr('明るい',false), dr('忙しい',true),
      dr('静か',true), dr('豊か',false), dr('焼く',true), dr('触る',true), dr('見える',false), dr('向かう',true), dr('戻る',true), dr('焼く',false),
      dr('困る',true), dr('直す',false), dr('守る',true), dr('考える',true), dr('決める',false), dr('逃げる',true), dr('払う',true), dr('続ける',false),
      dr('信じる',true), dr('調べる',true), dr('手伝う',false), dr('並ぶ',true), dr('脱ぐ',false), dr('登る',true), dr('減る',true), dr('曲がる',false),
    ],
    added: [
      wr('不便'), wr('天気'), wr('経験'), wr('練習'), wr('準備'), wr('心配'), wr('問題'), wr('機会'),
      wr('記念'), wr('原因'), wr('事故'), wr('景色'), wr('旅行'), wr('文化'), wr('説明'), wr('相談'),
      wr('日常'), wr('理由'),
    ],
    cleared: [wr('静か'), wr('豊か'), wr('大切'), wr('必要'), wr('安全'), wr('便利')],
  },
  '2025-11-17': {
    drilled: [dr('不便',false), dr('天気',true), dr('経験',true), dr('練習',false), dr('真剣',true), dr('正確',true), dr('悔しい',true), dr('便利',true)],
    added: [], cleared: [],
  },
  '2025-11-19': {
    drilled: [dr('準備',true), dr('心配',false), dr('問題',true), dr('機会',true), dr('天気',true), dr('経験',true), dr('不便',true), dr('練習',true)],
    added: [wr('準備'), wr('心配'), wr('問題'), wr('機会')],
    cleared: [],
  },
  '2025-11-21': {
    drilled: [dr('記念',true), dr('原因',false), dr('事故',true), dr('準備',true), dr('問題',true), dr('機会',false), dr('正しい',true), dr('安全',true)],
    added: [wr('記念'), wr('原因'), wr('事故')],
    cleared: [wr('考える'), wr('決める')],
  },
  '2025-11-24': {
    drilled: [dr('景色',true), dr('旅行',false), dr('文化',true), dr('記念',true), dr('原因',true), dr('事故',false), dr('天気',true), dr('練習',true)],
    added: [wr('景色'), wr('旅行'), wr('文化')],
    cleared: [],
  },
  '2025-11-26': {
    drilled: [dr('景色',true), dr('旅行',true), dr('文化',true), dr('心配',true), dr('問題',false), dr('機会',true), dr('厳しい',true), dr('珍しい',true)],
    added: [], cleared: [],
  },
  '2025-11-28': {
    drilled: [dr('説明',true), dr('相談',false), dr('旅行',true), dr('文化',true), dr('記念',true), dr('経験',false), dr('練習',true), dr('必要',true)],
    added: [wr('説明'), wr('相談')],
    cleared: [wr('守る'), wr('見える'), wr('直す')],
  },
};

// ── Calendar utilities ────────────────────────────────────────────────────────

function weekSunday(dateStr) {
  const d = new Date(dateStr + 'T00:00:00');
  const sun = new Date(d);
  sun.setDate(d.getDate() - d.getDay()); // getDay(): 0=Sun, 1=Mon, …
  return sun;
}

function toDateStr(d) { return d.toISOString().slice(0, 10); }

function addDays(d, n) {
  const r = new Date(d);
  r.setDate(r.getDate() + n);
  return r;
}

const INITIAL_WEEKS = 5;
const LOAD_BATCH = 4;
let weeksLoaded = 0;

function appendWeeks(count) {
  const cal = document.getElementById('calendar');
  const sun = weekSunday(TODAY);
  let added = 0;
  let exhausted = false;

  while (added < count) {
    const start = addDays(sun, -weeksLoaded * 7);
    if (toDateStr(start) < HISTORY_START) { exhausted = true; break; }
    const weekDays = Array.from({length: 7}, (_, j) => toDateStr(addDays(start, j)));
    const row = document.createElement('div');
    row.className = 'week-row';
    weekDays.forEach(dateStr => row.appendChild(buildDayCell(dateStr)));
    cal.appendChild(row);
    weeksLoaded++;
    added++;
  }

  return exhausted;
}

function formatDateFull(dateStr) {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-GB', {
    weekday: 'long', day: 'numeric', month: 'long', year: 'numeric'
  });
}

function dayLabel(dateStr) {
  const d = new Date(dateStr + 'T00:00:00');
  if (d.getDay() === 0) return d.toLocaleDateString('en-GB', {month: 'short', day: 'numeric'});
  return String(d.getDate());
}

// ── Stats ─────────────────────────────────────────────────────────────────────

const stats = {
  lexiconSize:      96,
  activeWords:      72,
  clearedLifetime:  28,
  streakDays:        7,
  avgClearedPerDay:  1.3,
  avgPerDay:        11.4,
  avgAddedPerDay:    0.6,
  daysActiveMonth:  18,
  // progress bar segments
  drillsCleared:    84,   // words that reached their target
  drillsClose:      38,   // active, <= 4 drills remaining
  drillsMid:        57,   // active, <= 8 drills remaining
  drillsFar:        53,   // active, > 8 drills remaining
  drillsRemaining: 128,   // total drills still needed across all active words
};

function renderStats() {
  const el = document.getElementById('stats-section');

  const wordTotal = stats.drillsCleared + stats.drillsClose + stats.drillsMid + stats.drillsFar;
  const pct = n => (n / wordTotal * 100).toFixed(1);

  el.innerHTML = `
    <div class="stat-grid">
      <div class="stat-card" data-tooltip="Total words in your vocabulary">
        <div class="stat-value">${stats.lexiconSize}</div>
        <div class="stat-label">Words in lexicon</div>
      </div>
      <div class="stat-card" data-tooltip="Words below their target drill count&#10;Eligible to be drawn in drills">
        <div class="stat-value">${stats.activeWords}</div>
        <div class="stat-label">Active words</div>
      </div>
      <div class="stat-card" data-tooltip="Words that have reached their target&#10;drill count at least once">
        <div class="stat-value">${stats.clearedLifetime}</div>
        <div class="stat-label">Cleared (lifetime)</div>
      </div>
      <div class="stat-card" data-tooltip="Words cleared per active day&#10;Last 7 days: 1.7&#10;Last 30 days: 1.3&#10;All time: 0.9">
        <div class="stat-value">${stats.avgClearedPerDay}</div>
        <div class="stat-label">Avg cleared per day</div>
      </div>
      <div class="stat-card" data-tooltip="Words drilled per active day&#10;Last 7 days: 13.1&#10;Last 30 days: 11.4&#10;All time: 10.2">
        <div class="stat-value">${stats.avgPerDay}</div>
        <div class="stat-label">Avg drilled per day</div>
      </div>
      <div class="stat-card" data-tooltip="Words added per active day&#10;Last 7 days: 0.4&#10;Last 30 days: 0.6&#10;All time: 0.5">
        <div class="stat-value">${stats.avgAddedPerDay}</div>
        <div class="stat-label">Avg added per day</div>
      </div>
    </div>
    <div class="drill-progress">
      <div class="drill-progress-label">Words by drills remaining to target</div>
      <div class="drill-progress-track">
        <div class="drill-progress-seg seg-cleared" style="width:${pct(stats.drillsCleared)}%"></div>
        <div class="drill-progress-seg seg-close"   style="width:${pct(stats.drillsClose)}%"></div>
        <div class="drill-progress-seg seg-mid"     style="width:${pct(stats.drillsMid)}%"></div>
        <div class="drill-progress-seg seg-far"     style="width:${pct(stats.drillsFar)}%"></div>
      </div>
      <div class="drill-progress-legend">
        <span class="legend-item legend-cleared" data-tooltip="Reached target drill count">&#9632; ${stats.drillsCleared} 🍏</span>
        <span class="legend-item legend-close" data-tooltip="4 drills or fewer remaining to target">&#9632; ${stats.drillsClose} 🌳</span>
        <span class="legend-item legend-mid" data-tooltip="8 drills or fewer remaining to target">&#9632; ${stats.drillsMid} 🌿</span>
        <span class="legend-item legend-far" data-tooltip="more than 8 drills remaining to target">&#9632; ${stats.drillsFar} 🌱</span>
      </div>
    </div>`;
}

// ── Rendering ─────────────────────────────────────────────────────────────────

function renderCalendar() {
  const labelsEl = document.getElementById('day-labels');
  ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'].forEach(name => {
    const div = document.createElement('div');
    div.className = 'day-label';
    div.textContent = name;
    labelsEl.appendChild(div);
  });

  appendWeeks(INITIAL_WEEKS);

  const wrap = document.querySelector('.calendar-wrap');

  const endBar = document.createElement('div');
  endBar.className = 'calendar-end hidden';
  endBar.textContent = 'Beginning of history';

  wrap.appendChild(endBar);

  const activityBody = document.querySelector('.activity-body');
  let exhausted = false;

  activityBody.addEventListener('scroll', () => {
    if (exhausted) return;
    const { scrollTop, scrollHeight, clientHeight } = activityBody;
    if (scrollTop + clientHeight >= scrollHeight - 200) {
      exhausted = appendWeeks(LOAD_BATCH);
      if (exhausted) endBar.classList.remove('hidden');
    }
  });
}

function buildDayCell(dateStr) {
  const cell = document.createElement('div');
  cell.className = 'day-cell';

  const isFuture = dateStr > TODAY;
  const isToday = dateStr === TODAY;
  const data = activityData[dateStr];

  if (isFuture) cell.classList.add('future');
  if (isToday) cell.classList.add('today');

  const numEl = document.createElement('div');
  numEl.className = 'day-number';
  numEl.textContent = dayLabel(dateStr);
  cell.appendChild(numEl);

  if (data && !isFuture) {
    cell.classList.add('has-activity');
    cell.addEventListener('click', () => openDayModal(dateStr));

    const badges = document.createElement('div');
    badges.className = 'day-badges';

    if (data.drilled.length) {
      const knew = data.drilled.filter(e => e.knew).length;
      const missed = data.drilled.length - knew;
      if (knew)   badges.appendChild(makeBadge('drilled-knew',   knew   + ' drilled ✓'));
      if (missed) badges.appendChild(makeBadge('drilled-missed', missed + ' drilled ✗'));
    }
    if (data.added.length)   badges.appendChild(makeBadge('added',   data.added.length + ' added'));
    if (data.cleared.length) badges.appendChild(makeBadge('cleared', data.cleared.length + ' cleared'));

    cell.appendChild(badges);
  }

  return cell;
}

function makeBadge(type, text) {
  const div = document.createElement('div');
  div.className = 'day-badge badge-' + type;
  const dot = document.createElement('span');
  dot.className = 'badge-dot';
  const label = document.createTextNode(text);
  div.appendChild(dot);
  div.appendChild(label);
  return div;
}

// ── Modal ─────────────────────────────────────────────────────────────────────

function openDayModal(dateStr) {
  const data = activityData[dateStr];
  document.getElementById('day-modal-title').textContent = formatDateFull(dateStr);
  const body = document.getElementById('day-modal-body');
  body.innerHTML = '';

  if (data.drilled.length) {
    const knew = data.drilled.filter(e => e.knew).length;
    const missed = data.drilled.length - knew;
    body.appendChild(buildSection(
      'Drilled — ' + knew + ' ✓  ' + missed + ' ✗',
      data.drilled,
      'drilled'
    ));
  }
  if (data.added.length)   body.appendChild(buildSection('Added',   data.added,   'added'));
  if (data.cleared.length) body.appendChild(buildSection('Cleared', data.cleared, 'cleared'));

  document.getElementById('day-modal-backdrop').classList.remove('hidden');
}

function buildSection(title, words, type) {
  const section = document.createElement('div');
  const titleEl = document.createElement('div');
  titleEl.className = 'day-section-title';
  titleEl.textContent = title;
  section.appendChild(titleEl);

  const list = document.createElement('div');
  list.className = 'day-word-list';

  words.forEach(entry => {
    const item = document.createElement('div');
    item.className = 'day-word-item';
    item.innerHTML =
      '<span class="day-word-jp">' + entry.word + '</span>' +
      '<span class="day-word-reading">' + entry.reading + '</span>' +
      '<span class="day-word-meaning">' + entry.meaning + '</span>' +
      (type === 'drilled'
        ? '<span class="day-word-result ' + (entry.knew ? 'knew' : 'missed') + '">' + (entry.knew ? '✓' : '✗') + '</span>'
        : '');
    list.appendChild(item);
  });

  section.appendChild(list);
  return section;
}

function closeDayModal() {
  document.getElementById('day-modal-backdrop').classList.add('hidden');
}

function handleDayBackdropClick(e) {
  if (e.target === document.getElementById('day-modal-backdrop')) closeDayModal();
}

document.addEventListener('keydown', e => { if (e.key === 'Escape') closeDayModal(); });

// ── Tooltip ───────────────────────────────────────────────────────────────────

const actTooltip = document.createElement('div');
actTooltip.className = 'lex-tooltip';
document.body.appendChild(actTooltip);

document.addEventListener('mouseover', e => {
  const target = e.target.closest('[data-tooltip]');
  if (!target) return;
  actTooltip.textContent = target.dataset.tooltip;
  actTooltip.classList.add('visible');
});

document.addEventListener('mousemove', e => {
  if (!actTooltip.classList.contains('visible')) return;
  const x = e.clientX + 14;
  actTooltip.style.top  = (e.clientY + 14) + 'px';
  actTooltip.style.left = (x + actTooltip.offsetWidth > window.innerWidth)
    ? (e.clientX - actTooltip.offsetWidth) + 'px'
    : x + 'px';
});

document.addEventListener('mouseout', e => {
  if (!e.target.closest('[data-tooltip]')) return;
  actTooltip.classList.remove('visible');
});

renderStats();
renderCalendar();

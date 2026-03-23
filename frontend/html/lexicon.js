const words = [
  { word: '走る', reading: 'はしる', meaning: 'to run', type: 'godan-verb', exampleJp: '公園を走る。', exampleEn: 'I run in the park.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '美しい', reading: 'うつくしい', meaning: 'beautiful', type: 'i-adjective', exampleJp: '美しい花が咲いている。', exampleEn: 'Beautiful flowers are blooming.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '話す', reading: 'はなす', meaning: 'to speak; to talk', type: 'godan-verb', exampleJp: '日本語を話す。', exampleEn: 'I speak Japanese.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '静か', reading: 'しずか', meaning: 'quiet', type: 'na-adjective', exampleJp: '図書館は静かだ。', exampleEn: 'The library is quiet.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '始める', reading: 'はじめる', meaning: 'to begin; to start', type: 'ichidan-verb', exampleJp: '仕事を始める。', exampleEn: 'I start work.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '天気', reading: 'てんき', meaning: 'weather', type: 'noun', exampleJp: '今日は天気がいい。', exampleEn: 'The weather is nice today.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '忘れる', reading: 'わすれる', meaning: 'to forget', type: 'ichidan-verb', exampleJp: '名前を忘れる。', exampleEn: 'I forget the name.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '危ない', reading: 'あぶない', meaning: 'dangerous', type: 'i-adjective', exampleJp: 'この道は危ない。', exampleEn: 'This road is dangerous.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '選ぶ', reading: 'えらぶ', meaning: 'to choose; to select', type: 'godan-verb', exampleJp: '好きな色を選ぶ。', exampleEn: 'I choose my favorite color.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '問題', reading: 'もんだい', meaning: 'problem; question', type: 'noun', exampleJp: '問題を解く。', exampleEn: 'I solve the problem.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '暖かい', reading: 'あたたかい', meaning: 'warm', type: 'i-adjective', exampleJp: '今日は暖かい。', exampleEn: 'It is warm today.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '借りる', reading: 'かりる', meaning: 'to borrow', type: 'ichidan-verb', exampleJp: '本を借りる。', exampleEn: 'I borrow a book.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '経験', reading: 'けいけん', meaning: 'experience', type: 'noun', exampleJp: 'いい経験になった。', exampleEn: 'It became a good experience.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '集める', reading: 'あつめる', meaning: 'to collect; to gather', type: 'ichidan-verb', exampleJp: '切手を集める。', exampleEn: 'I collect stamps.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '複雑', reading: 'ふくざつ', meaning: 'complicated; complex', type: 'na-adjective', exampleJp: 'この地図は複雑だ。', exampleEn: 'This map is complicated.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '届く', reading: 'とどく', meaning: 'to arrive; to reach', type: 'godan-verb', exampleJp: '手紙が届く。', exampleEn: 'The letter arrives.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '練習', reading: 'れんしゅう', meaning: 'practice', type: 'noun', exampleJp: '毎日練習する。', exampleEn: 'I practice every day.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '眠い', reading: 'ねむい', meaning: 'sleepy', type: 'i-adjective', exampleJp: 'とても眠い。', exampleEn: 'I am very sleepy.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '伝える', reading: 'つたえる', meaning: 'to convey; to tell', type: 'ichidan-verb', exampleJp: '気持ちを伝える。', exampleEn: 'I convey my feelings.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '丁寧', reading: 'ていねい', meaning: 'polite; careful', type: 'na-adjective', exampleJp: '丁寧に説明する。', exampleEn: 'I explain politely.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '探す', reading: 'さがす', meaning: 'to search; to look for', type: 'godan-verb', exampleJp: '鍵を探す。', exampleEn: 'I look for the key.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '景色', reading: 'けしき', meaning: 'scenery; view', type: 'noun', exampleJp: '景色がきれいだ。', exampleEn: 'The scenery is beautiful.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '壊れる', reading: 'こわれる', meaning: 'to break (intransitive)', type: 'ichidan-verb', exampleJp: '時計が壊れる。', exampleEn: 'The clock breaks.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '残念', reading: 'ざんねん', meaning: 'regrettable; unfortunate', type: 'na-adjective', exampleJp: '残念な結果だった。', exampleEn: 'It was an unfortunate result.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '誘う', reading: 'さそう', meaning: 'to invite', type: 'godan-verb', exampleJp: '友達を誘う。', exampleEn: 'I invite a friend.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '約束', reading: 'やくそく', meaning: 'promise; appointment', type: 'noun', exampleJp: '約束を守る。', exampleEn: 'I keep the promise.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '優しい', reading: 'やさしい', meaning: 'kind; gentle', type: 'i-adjective', exampleJp: '彼は優しい人だ。', exampleEn: 'He is a kind person.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '受ける', reading: 'うける', meaning: 'to receive; to take (exam)', type: 'ichidan-verb', exampleJp: '試験を受ける。', exampleEn: 'I take an exam.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '自由', reading: 'じゆう', meaning: 'freedom; free', type: 'na-adjective', exampleJp: '自由な時間が欲しい。', exampleEn: 'I want free time.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '捨てる', reading: 'すてる', meaning: 'to throw away', type: 'ichidan-verb', exampleJp: 'ゴミを捨てる。', exampleEn: 'I throw away the trash.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '泳ぐ', reading: 'およぐ', meaning: 'to swim', type: 'godan-verb', exampleJp: '海で泳ぐ。', exampleEn: 'I swim in the sea.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '運ぶ', reading: 'はこぶ', meaning: 'to carry', type: 'godan-verb', exampleJp: '荷物を運ぶ。', exampleEn: 'I carry the luggage.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '起きる', reading: 'おきる', meaning: 'to wake up; to get up', type: 'ichidan-verb', exampleJp: '早く起きる。', exampleEn: 'I wake up early.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '落ちる', reading: 'おちる', meaning: 'to fall', type: 'ichidan-verb', exampleJp: '葉が落ちる。', exampleEn: 'The leaves fall.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '変える', reading: 'かえる', meaning: 'to change', type: 'ichidan-verb', exampleJp: '計画を変える。', exampleEn: 'I change the plan.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '考える', reading: 'かんがえる', meaning: 'to think; to consider', type: 'ichidan-verb', exampleJp: '答えを考える。', exampleEn: 'I think of the answer.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '決める', reading: 'きめる', meaning: 'to decide', type: 'ichidan-verb', exampleJp: '目標を決める。', exampleEn: 'I decide on a goal.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '聞こえる', reading: 'きこえる', meaning: 'to be audible; to hear', type: 'ichidan-verb', exampleJp: '音楽が聞こえる。', exampleEn: 'I can hear the music.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '比べる', reading: 'くらべる', meaning: 'to compare', type: 'ichidan-verb', exampleJp: '値段を比べる。', exampleEn: 'I compare the prices.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '困る', reading: 'こまる', meaning: 'to be troubled; to be in a fix', type: 'godan-verb', exampleJp: '道に迷って困る。', exampleEn: 'I am troubled after getting lost.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '転ぶ', reading: 'ころぶ', meaning: 'to fall down; to tumble', type: 'godan-verb', exampleJp: '滑って転ぶ。', exampleEn: 'I slip and fall down.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '触る', reading: 'さわる', meaning: 'to touch', type: 'godan-verb', exampleJp: '猫に触る。', exampleEn: 'I touch the cat.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '調べる', reading: 'しらべる', meaning: 'to investigate; to look up', type: 'ichidan-verb', exampleJp: '言葉を調べる。', exampleEn: 'I look up the word.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '信じる', reading: 'しんじる', meaning: 'to believe', type: 'ichidan-verb', exampleJp: '友達を信じる。', exampleEn: 'I believe my friend.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '育てる', reading: 'そだてる', meaning: 'to raise; to bring up', type: 'ichidan-verb', exampleJp: '子供を育てる。', exampleEn: 'I raise a child.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '続ける', reading: 'つづける', meaning: 'to continue', type: 'ichidan-verb', exampleJp: '練習を続ける。', exampleEn: 'I continue practicing.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '手伝う', reading: 'てつだう', meaning: 'to help; to assist', type: 'godan-verb', exampleJp: '母を手伝う。', exampleEn: 'I help my mother.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '直す', reading: 'なおす', meaning: 'to fix; to correct', type: 'godan-verb', exampleJp: '時計を直す。', exampleEn: 'I fix the clock.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '並ぶ', reading: 'ならぶ', meaning: 'to line up; to stand in a row', type: 'godan-verb', exampleJp: '列に並ぶ。', exampleEn: 'I line up in the queue.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '逃げる', reading: 'にげる', meaning: 'to run away; to escape', type: 'ichidan-verb', exampleJp: '危険から逃げる。', exampleEn: 'I escape from danger.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '脱ぐ', reading: 'ぬぐ', meaning: 'to take off (clothes)', type: 'godan-verb', exampleJp: '靴を脱ぐ。', exampleEn: 'I take off my shoes.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '登る', reading: 'のぼる', meaning: 'to climb', type: 'godan-verb', exampleJp: '山に登る。', exampleEn: 'I climb the mountain.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '払う', reading: 'はらう', meaning: 'to pay', type: 'godan-verb', exampleJp: '料金を払う。', exampleEn: 'I pay the fee.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '減る', reading: 'へる', meaning: 'to decrease; to diminish', type: 'godan-verb', exampleJp: '体重が減る。', exampleEn: 'My weight decreases.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '曲がる', reading: 'まがる', meaning: 'to turn; to bend', type: 'godan-verb', exampleJp: '角を曲がる。', exampleEn: 'I turn the corner.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '守る', reading: 'まもる', meaning: 'to protect; to keep (a promise)', type: 'godan-verb', exampleJp: 'ルールを守る。', exampleEn: 'I follow the rules.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '見える', reading: 'みえる', meaning: 'to be visible; can see', type: 'ichidan-verb', exampleJp: '海が見える。', exampleEn: 'I can see the sea.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '向かう', reading: 'むかう', meaning: 'to head toward; to face', type: 'godan-verb', exampleJp: '駅に向かう。', exampleEn: 'I head toward the station.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '戻る', reading: 'もどる', meaning: 'to return; to go back', type: 'godan-verb', exampleJp: '家に戻る。', exampleEn: 'I return home.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '焼く', reading: 'やく', meaning: 'to bake; to grill', type: 'godan-verb', exampleJp: 'パンを焼く。', exampleEn: 'I bake bread.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '意見', reading: 'いけん', meaning: 'opinion', type: 'noun', exampleJp: '意見を言う。', exampleEn: 'I state my opinion.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '機会', reading: 'きかい', meaning: 'opportunity; chance', type: 'noun', exampleJp: '機会を活かす。', exampleEn: 'I make use of the opportunity.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '記念', reading: 'きねん', meaning: 'commemoration; memory', type: 'noun', exampleJp: '記念写真を撮る。', exampleEn: 'I take a commemorative photo.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '計画', reading: 'けいかく', meaning: 'plan', type: 'noun', exampleJp: '計画を立てる。', exampleEn: 'I make a plan.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '原因', reading: 'げんいん', meaning: 'cause; reason', type: 'noun', exampleJp: '事故の原因を調べる。', exampleEn: 'I investigate the cause of the accident.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '事故', reading: 'じこ', meaning: 'accident', type: 'noun', exampleJp: '交通事故が起きた。', exampleEn: 'A traffic accident occurred.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '準備', reading: 'じゅんび', meaning: 'preparation', type: 'noun', exampleJp: '準備ができた。', exampleEn: 'The preparations are done.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '場所', reading: 'ばしょ', meaning: 'place; location', type: 'noun', exampleJp: '待ち合わせの場所を決める。', exampleEn: 'I decide on a meeting place.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '心配', reading: 'しんぱい', meaning: 'worry; concern', type: 'noun', exampleJp: '心配をかけてごめん。', exampleEn: 'Sorry for causing worry.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '生活', reading: 'せいかつ', meaning: 'daily life; living', type: 'noun', exampleJp: '生活が豊かになった。', exampleEn: 'My life became rich.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '説明', reading: 'せつめい', meaning: 'explanation', type: 'noun', exampleJp: '説明を聞く。', exampleEn: 'I listen to the explanation.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '相談', reading: 'そうだん', meaning: 'consultation; discussion', type: 'noun', exampleJp: '先生に相談する。', exampleEn: 'I consult with the teacher.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '注意', reading: 'ちゅうい', meaning: 'caution; attention', type: 'noun', exampleJp: '注意して運転する。', exampleEn: 'I drive carefully.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '日常', reading: 'にちじょう', meaning: 'everyday; daily life', type: 'noun', exampleJp: '日常の生活を大切にする。', exampleEn: 'I value everyday life.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '費用', reading: 'ひよう', meaning: 'cost; expense', type: 'noun', exampleJp: '費用を計算する。', exampleEn: 'I calculate the cost.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '文化', reading: 'ぶんか', meaning: 'culture', type: 'noun', exampleJp: '日本の文化を学ぶ。', exampleEn: 'I learn about Japanese culture.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '方法', reading: 'ほうほう', meaning: 'method; way', type: 'noun', exampleJp: '勉強の方法を変える。', exampleEn: 'I change my study method.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '理由', reading: 'りゆう', meaning: 'reason', type: 'noun', exampleJp: '理由を説明する。', exampleEn: 'I explain the reason.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '旅行', reading: 'りょこう', meaning: 'travel; trip', type: 'noun', exampleJp: '旅行の計画を立てる。', exampleEn: 'I make travel plans.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '明るい', reading: 'あかるい', meaning: 'bright; cheerful', type: 'i-adjective', exampleJp: '部屋が明るい。', exampleEn: 'The room is bright.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '怪しい', reading: 'あやしい', meaning: 'suspicious; strange', type: 'i-adjective', exampleJp: '怪しい人がいる。', exampleEn: 'There is a suspicious person.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '忙しい', reading: 'いそがしい', meaning: 'busy', type: 'i-adjective', exampleJp: '毎日忙しい。', exampleEn: 'I am busy every day.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '嬉しい', reading: 'うれしい', meaning: 'happy; glad', type: 'i-adjective', exampleJp: '合格して嬉しい。', exampleEn: 'I am happy to have passed.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '悲しい', reading: 'かなしい', meaning: 'sad', type: 'i-adjective', exampleJp: '別れが悲しい。', exampleEn: 'The farewell is sad.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '難しい', reading: 'むずかしい', meaning: 'difficult', type: 'i-adjective', exampleJp: 'この問題は難しい。', exampleEn: 'This problem is difficult.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '珍しい', reading: 'めずらしい', meaning: 'rare; unusual', type: 'i-adjective', exampleJp: '珍しい鳥を見た。', exampleEn: 'I saw a rare bird.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '厳しい', reading: 'きびしい', meaning: 'strict; severe', type: 'i-adjective', exampleJp: '先生が厳しい。', exampleEn: 'The teacher is strict.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '悔しい', reading: 'くやしい', meaning: 'frustrated; vexed', type: 'i-adjective', exampleJp: '負けて悔しい。', exampleEn: 'I feel frustrated after losing.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '正しい', reading: 'ただしい', meaning: 'correct; right', type: 'i-adjective', exampleJp: '正しい答えを選ぶ。', exampleEn: 'I choose the correct answer.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '便利', reading: 'べんり', meaning: 'convenient', type: 'na-adjective', exampleJp: 'スマホは便利だ。', exampleEn: 'Smartphones are convenient.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
  { word: '不便', reading: 'ふべん', meaning: 'inconvenient', type: 'na-adjective', exampleJp: '田舎は不便だ。', exampleEn: 'The countryside is inconvenient.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-03-22' },
  { word: '正確', reading: 'せいかく', meaning: 'accurate; precise', type: 'na-adjective', exampleJp: '正確な情報が必要だ。', exampleEn: 'Accurate information is needed.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: null },
  { word: '大切', reading: 'たいせつ', meaning: 'important; precious', type: 'na-adjective', exampleJp: '時間を大切にする。', exampleEn: 'I value my time.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: '2026-03-10' },
  { word: '必要', reading: 'ひつよう', meaning: 'necessary', type: 'na-adjective', exampleJp: '努力が必要だ。', exampleEn: 'Effort is necessary.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-03-20' },
  { word: '豊か', reading: 'ゆたか', meaning: 'rich; abundant', type: 'na-adjective', exampleJp: '自然が豊かな国だ。', exampleEn: 'It is a country rich in nature.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: null },
  { word: '適切', reading: 'てきせつ', meaning: 'appropriate; suitable', type: 'na-adjective', exampleJp: '適切な言葉を選ぶ。', exampleEn: 'I choose appropriate words.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-23', lastDrilled: '2026-02-15' },
  { word: '十分', reading: 'じゅうぶん', meaning: 'sufficient; enough', type: 'na-adjective', exampleJp: '準備は十分だ。', exampleEn: 'The preparation is sufficient.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-18', lastDrilled: '2026-03-23' },
  { word: '真剣', reading: 'しんけん', meaning: 'serious; earnest', type: 'na-adjective', exampleJp: '真剣に考える。', exampleEn: 'I think seriously.', correct: 0, incorrect: 0, target: 3, createdAt: '2026-03-01', lastDrilled: null },
  { word: '安全', reading: 'あんぜん', meaning: 'safe; secure', type: 'na-adjective', exampleJp: 'この道は安全だ。', exampleEn: 'This road is safe.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-12-01', lastDrilled: '2026-01-08' },
  { word: '特別', reading: 'とくべつ', meaning: 'special', type: 'na-adjective', exampleJp: '今日は特別な日だ。', exampleEn: 'Today is a special day.', correct: 0, incorrect: 0, target: 3, createdAt: '2025-06-01', lastDrilled: '2026-03-18' },
];

function updateWordCount() {
  const active = words.filter(w => w.correct < w.target).length;
  document.getElementById('word-count').textContent =
    words.length + ' words (' + active + ' active)';
}
updateWordCount();

const typeLabels = {
  'godan-verb':   'Godan verb — Group 1 (五段動詞)',
  'ichidan-verb': 'Ichidan verb — Group 2 (一段動詞)',
  'noun':         'Noun (名詞)',
  'i-adjective':  'い-adjective (い形容詞)',
  'na-adjective': 'な-adjective (な形容詞)',
  'adverb':       'Adverb (副詞)',
};

function timeAgo(dateStr) {
  const sec = Math.floor((Date.now() - new Date(dateStr)) / 1000);
  const min = Math.floor(sec / 60);
  if (min < 1) return 'just now';
  if (min < 60)   return min + ' minute' + (min === 1 ? '' : 's') + ' ago';
  const hr = Math.floor(min / 60);
  if (hr < 24)    return hr + ' hour' + (hr === 1 ? '' : 's') + ' ago';
  const day = Math.floor(hr / 24);
  if (day < 30)   return day + ' day' + (day === 1 ? '' : 's') + ' ago';
  const mo = Math.floor(day / 30);
  if (mo < 12)    return mo + ' month' + (mo === 1 ? '' : 's') + ' ago';
  const yr = Math.floor(day / 365);
  return yr + ' year' + (yr === 1 ? '' : 's') + ' ago';
}

function fullDateTime(dateStr) {
  return new Date(dateStr).toLocaleString(undefined, {
    year: 'numeric', month: 'long', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

function renderRow(w, trMain, trEx) {
  trMain.innerHTML =
    '<td><div class="cell-word" title="Japanese word">' + w.word +
      '<button class="btn-edit" onclick="openModal(event)" title="Edit word">✎</button>' +
    '</div></td>' +
    '<td class="cell-reading" title="Hiragana reading">' + w.reading + '</td>' +
    '<td title="Part of speech"><span class="type-badge" title="' + (typeLabels[w.type] || w.type) + '">' + w.type + '</span></td>' +
    '<td class="cell-meaning" title="English meaning">' + w.meaning + '</td>' +
    '<td class="cell-correct" title="Times answered correctly">' + w.correct + '</td>' +
    '<td class="cell-incorrect" title="Times answered incorrectly">' + w.incorrect + '</td>' +
    '<td class="cell-target" title="Target number of additional drills">' + w.target + '</td>';
  trMain._word = w;
  trMain._trEx  = trEx;

  trEx.innerHTML =
    '<td colspan="2" class="cell-date">' +
      '<span class="cell-date-added" title="Date added: ' + fullDateTime(w.createdAt) + '">added ' + timeAgo(w.createdAt) + '</span>' +
      '<span class="cell-date-sep"> · </span>' +
      (w.lastDrilled
        ? '<span class="cell-date-drilled" title="Last drilled: ' + fullDateTime(w.lastDrilled) + '">drilled ' + timeAgo(w.lastDrilled) + '</span>'
        : '<span class="cell-date-drilled cell-date-never">never drilled</span>') +
    '</td>' +
    '<td colspan="5" class="cell-ex">' +
      '<span class="cell-ex-jp" title="Example sentence (Japanese)">' + w.exampleJp + '</span> ' +
      '<span class="cell-ex-en" title="Example sentence (English)">' + w.exampleEn + '</span>' +
    '</td>';
}

const tbody = document.getElementById('word-tbody');
words.forEach(w => {
  const trMain = document.createElement('tr');
  trMain.className = 'row-main';
  const trEx = document.createElement('tr');
  trEx.className = 'row-example';
  renderRow(w, trMain, trEx);
  tbody.appendChild(trMain);
  tbody.appendChild(trEx);
});

// --- Modal ---
let _modalTrMain = null;

function openModal(event) {
  event.stopPropagation();
  const trMain = event.target.closest('tr');
  _modalTrMain = trMain;
  const w = trMain._word;
  document.getElementById('modal-word-label').textContent = w.word;
  document.getElementById('edit-reading').value  = w.reading;
  document.getElementById('edit-type').value     = w.type;
  document.getElementById('edit-meaning').value  = w.meaning;
  document.getElementById('edit-ex-jp').value    = w.exampleJp;
  document.getElementById('edit-ex-en').value    = w.exampleEn;
  document.getElementById('edit-target').value   = w.target;
  document.getElementById('modal-backdrop').classList.remove('hidden');
}

function closeModal() {
  document.getElementById('modal-backdrop').classList.add('hidden');
}

function handleBackdropClick(event) {
  if (event.target === document.getElementById('modal-backdrop')) closeModal();
}

function saveModal() {
  const w = _modalTrMain._word;
  w.reading   = document.getElementById('edit-reading').value;
  w.type      = document.getElementById('edit-type').value;
  w.meaning   = document.getElementById('edit-meaning').value;
  w.exampleJp = document.getElementById('edit-ex-jp').value;
  w.exampleEn = document.getElementById('edit-ex-en').value;
  w.target    = parseInt(document.getElementById('edit-target').value, 10);
  renderRow(w, _modalTrMain, _modalTrMain._trEx);
  closeModal();
}

function adjustTarget(delta) {
  const input = document.getElementById('edit-target');
  input.value = Math.max(0, (parseInt(input.value, 10) || 0) + delta);
}

document.addEventListener('keydown', e => { if (e.key === 'Escape') { closeModal(); closeAddModal(); } });

// --- Add words modal ---
function openAddModal() {
  document.getElementById('add-words-input').value = '';
  document.getElementById('add-modal-backdrop').classList.remove('hidden');
  document.getElementById('add-words-input').focus();
}

function closeAddModal() {
  document.getElementById('add-modal-backdrop').classList.add('hidden');
}

function handleAddBackdropClick(event) {
  if (event.target === document.getElementById('add-modal-backdrop')) closeAddModal();
}

function saveAddModal() {
  const lines = document.getElementById('add-words-input').value
    .split(/[\s,、。・;:!?()（）「」【】『』\[\]]+/)
    .map(t => t.trim()).filter(t => t.length > 0);

  const today = new Date().toISOString().slice(0, 10);
  lines.forEach(word => {
    if (words.some(w => w.word === word)) return; // basic duplicate check
    const w = {
      word, reading: '', type: 'noun', meaning: '',
      exampleJp: '', exampleEn: '',
      correct: 0, incorrect: 0, target: 3,
      createdAt: today, lastDrilled: null,
    };
    words.push(w);
    const trMain = document.createElement('tr');
    trMain.className = 'row-main';
    const trEx = document.createElement('tr');
    trEx.className = 'row-example';
    renderRow(w, trMain, trEx);
    tbody.appendChild(trMain);
    tbody.appendChild(trEx);
  });

  updateWordCount();
  closeAddModal();
}

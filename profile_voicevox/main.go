// profile_voicevox measures VoiceVox synthesis performance across sentences of
// varying length, including Kagome tokenization timing.
//
// VoiceVox does NOT stream audio — it returns the complete WAV only after
// synthesis is fully done, so total round-trip time == minimum playback latency.
//
// Timings measured per sentence:
//
//   - KagomeMs: Kagome tokenization + clause-split analysis
//   - QueryMs:  VoiceVox audio_query call (NLP/phoneme analysis)
//   - SynthMs:  VoiceVox synthesis call (waveform generation + transfer)
//   - TotalMs:  KagomeMs + QueryMs + SynthMs
//
// Chunks shows how many clause fragments the sentence would be split into.
// If we pipeline VoiceVox requests per chunk, the effective per-chunk latency
// is roughly TotalMs / Chunks.
//
// Run with VoiceVox running at http://localhost:50021.
// Usage: go run . [--speaker N] [--repeat N]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	ipadict "github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"
)

const voicevoxBase = "http://localhost:50021"

// minChunkRunes is the minimum rune length a clause chunk must have. Chunks
// shorter than this are merged with the preceding chunk to avoid sending
// trivially short phrases like "て" as standalone synthesis requests.
const minChunkRunes = 6

// sentences is the sample corpus, ordered roughly by length / complexity.
var sentences = []string{
	// Very short — single words / short phrases
	"猫",
	"ありがとう",
	"おはようございます",
	"今日はいい天気ですね。",

	// Short sentences (5–10 words)
	"私は学生です。",
	"東京に住んでいます。",
	"毎朝コーヒーを飲みます。",
	"日本語を勉強しています。",
	"彼女は医者ではありません。",

	// Medium sentences (10–20 words)
	"昨日、友達と映画を見に行って、とても楽しかったです。",
	"この本はとても面白いので、ぜひ読んでみてください。",
	"電車が遅れたので、会議に少し遅刻してしまいました。",
	"春になると、桜の花が咲いてとても綺麗になります。",
	"彼は毎日六時に起きて、会社まで自転車で通っています。",

	// Longer sentences (20+ words)
	"先週の週末に家族と一緒に山登りをして、頂上からの景色がとても素晴らしかったので、また来年も行こうと思っています。",
	"日本の伝統的な文化には茶道や生け花など様々なものがありますが、最近の若者にはあまり知られていないのが残念です。",
	"新しいプロジェクトを始める前に、チーム全員で目標や役割分担を明確にしておくことがとても重要だと思います。",
	"この映画は原作の小説をとても忠実に再現しており、特に主人公の心理描写が細かくて、見ていてとても引き込まれました。",

	// Very long sentences
	"今年の夏休みは、家族みんなで沖縄に旅行する予定で、美しい海でシュノーケリングをしたり、地元の料理を楽しんだり、歴史的な場所を訪れたりして、忘れられない思い出を作りたいと思っています。",
	"科学技術の急速な発展により、私たちの日常生活は大きく変化しており、特に人工知能やロボット工学の分野では目覚ましい進歩が続いていますが、その一方で倫理的な問題や社会への影響についても真剣に考える必要があります。",

	// Dialogue / natural speech patterns
	"すみません、この近くに郵便局はありますか？",
	"ちょっと待ってください、すぐに戻ります。",
	"本当にありがとうございました。おかげさまで助かりました。",
	"明日の会議は何時から始まりますか？場所はどこですか？",
	"このレストランのパスタは本当においしいですね。また来ましょう。",
}

// splitClauses tokenizes text with Kagome and splits it into clause chunks at
// natural boundary points:
//   - after 読点 「、」 punctuation
//   - after 接続助詞 (conjunctive particles: て, で, が, けど, から, ので, ながら…)
//   - before 接続詞 (conjunctions that open a new clause: でも, しかし, だから…)
//
// Chunks shorter than minChunkRunes are merged with the preceding chunk to
// avoid sending trivially short phrases as standalone synthesis requests.
func splitClauses(text string, t *tokenizer.Tokenizer) []string {
	tokens := t.Tokenize(text)

	// Collect (surface, splitAfter) pairs. splitAfter=true means we cut
	// after appending this token's surface text to the current chunk.
	type tokenInfo struct {
		surface     string
		splitAfter  bool
		splitBefore bool // for 接続詞 — cut before this token
	}
	var infos []tokenInfo
	for _, tok := range tokens {
		f := tok.Features()
		pos := ""
		sub1 := ""
		if len(f) > 0 {
			pos = f[0]
		}
		if len(f) > 1 {
			sub1 = f[1]
		}

		info := tokenInfo{surface: tok.Surface}
		switch {
		case tok.Surface == "、" || tok.Surface == "。":
			info.splitAfter = true
		case pos == "助詞" && sub1 == "接続助詞":
			// て, で, が, けど, けれど, から, ので, ながら, し, たり, のに…
			// Exception: auxiliary verb chains (〜てください, 〜てみる, 〜ていく)
			// are handled naturally by minChunkRunes — if the following chunk
			// is too short it gets merged back.
			info.splitAfter = true
		case pos == "接続詞":
			// Conjunctions like でも, しかし, だから open a new clause.
			info.splitBefore = true
		}
		infos = append(infos, info)
	}

	// Build raw chunks from split markers.
	var chunks []string
	var cur strings.Builder
	for _, info := range infos {
		if info.splitBefore && cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
		cur.WriteString(info.surface)
		if info.splitAfter {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	if len(chunks) == 0 {
		return []string{text}
	}

	// Merge chunks that are too short into the preceding chunk.
	merged := []string{chunks[0]}
	for _, c := range chunks[1:] {
		if len([]rune(c)) < minChunkRunes {
			merged[len(merged)-1] += c
		} else {
			merged = append(merged, c)
		}
	}
	return merged
}

type result struct {
	text            string
	charCount       int
	kagomeMs        int64
	chunkCount      int
	chunks          []string
	queryMs         int64 // audio_query phase
	synthMs         int64 // synthesis phase (including transfer)
	totalMs         int64 // kagomeMs + queryMs + synthMs
	wavSizeBytes    int
	audioDurationMs int64
	err             error
}

// wavDurationMs reads the audio duration from a standard PCM WAV header.
func wavDurationMs(wav []byte) int64 {
	if len(wav) < 44 {
		return 0
	}
	byteRate := int64(uint32(wav[28]) | uint32(wav[29])<<8 | uint32(wav[30])<<16 | uint32(wav[31])<<24)
	dataSize := int64(uint32(wav[40]) | uint32(wav[41])<<8 | uint32(wav[42])<<16 | uint32(wav[43])<<24)
	if byteRate == 0 {
		return 0
	}
	return dataSize * 1000 / byteRate
}

func profileSentence(text string, speaker int, t *tokenizer.Tokenizer) result {
	r := result{text: text, charCount: len([]rune(text))}
	client := &http.Client{Timeout: 60 * time.Second}

	// Step 0: Kagome clause analysis
	kagomeStart := time.Now()
	chunks := splitClauses(text, t)
	r.kagomeMs = time.Since(kagomeStart).Milliseconds()
	r.chunks = chunks
	r.chunkCount = len(chunks)

	// Steps 1+2: profile only the first chunk for VoiceVox timing.
	// (Profiling all chunks would give a more complete picture but also
	// serializes the requests; chunk timings scale roughly linearly with
	// character count, so the first-chunk number is the most useful signal
	// for estimating "time until first audio starts".)
	synthesizeChunk := func(chunkText string) ([]byte, int64, int64, error) {
		start := time.Now()
		qURL := fmt.Sprintf("%s/audio_query?text=%s&speaker=%d",
			voicevoxBase, url.QueryEscape(chunkText), speaker)
		req, err := http.NewRequest(http.MethodPost, qURL, nil)
		if err != nil {
			return nil, 0, 0, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("audio_query: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, 0, 0, fmt.Errorf("audio_query status: %s", resp.Status)
		}
		var q map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
			return nil, 0, 0, fmt.Errorf("decode audio_query: %w", err)
		}
		qMs := time.Since(start).Milliseconds()

		qJSON, err := json.Marshal(q)
		if err != nil {
			return nil, 0, 0, err
		}
		sURL := fmt.Sprintf("%s/synthesis?speaker=%d", voicevoxBase, speaker)
		req2, err := http.NewRequest(http.MethodPost, sURL, bytes.NewReader(qJSON))
		if err != nil {
			return nil, 0, 0, err
		}
		req2.Header.Set("Content-Type", "application/json")
		synthStart := time.Now()
		resp2, err := client.Do(req2)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("synthesis: %w", err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			return nil, 0, 0, fmt.Errorf("synthesis status: %s", resp2.Status)
		}
		wav, err := io.ReadAll(resp2.Body)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("synthesis read: %w", err)
		}
		return wav, qMs, time.Since(synthStart).Milliseconds(), nil
	}

	wav, qMs, sMs, err := synthesizeChunk(chunks[0])
	if err != nil {
		r.err = err
		return r
	}
	r.queryMs = qMs
	r.synthMs = sMs
	r.wavSizeBytes = len(wav)
	r.audioDurationMs = wavDurationMs(wav)
	r.totalMs = r.kagomeMs + r.queryMs + r.synthMs
	return r
}

func avg(vals []int64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return float64(sum) / float64(len(vals))
}

func median(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]int64, len(vals))
	copy(cp, vals)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	n := len(cp)
	if n%2 == 0 {
		return (cp[n/2-1] + cp[n/2]) / 2
	}
	return cp[n/2]
}

func maxVal(vals []int64) int64 {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

func main() {
	speaker := flag.Int("speaker", 1, "VoiceVox speaker ID")
	repeat := flag.Int("repeat", 1, "number of times to run each sentence (results averaged)")
	showChunks := flag.Bool("chunks", false, "print the clause chunks for each sentence")
	flag.Parse()

	fmt.Printf("VoiceVox Profiler — speaker %d, %d repetition(s) per sentence\n", *speaker, *repeat)
	fmt.Printf("Base URL: %s\n\n", voicevoxBase)

	// Check VoiceVox is reachable.
	resp, err := http.Get(voicevoxBase + "/version")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot reach VoiceVox at %s: %v\n", voicevoxBase, err)
		os.Exit(1)
	}
	versionBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("VoiceVox version: %s\n\n", bytes.TrimSpace(versionBytes))

	// Initialize Kagome — measure one-time dict load cost.
	fmt.Print("Initializing Kagome IPA tokenizer... ")
	initStart := time.Now()
	t, err := tokenizer.New(ipadict.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("done (%d ms)\n\n", time.Since(initStart).Milliseconds())

	type agg struct {
		kagomeMs []int64
		queryMs  []int64
		synthMs  []int64
		totalMs  []int64
	}
	var results []result
	var all agg

	for i, sentence := range sentences {
		fmt.Printf("[%2d/%d] %s\n", i+1, len(sentences), sentence)

		var runs agg
		var last result
		for rep := 0; rep < *repeat; rep++ {
			r := profileSentence(sentence, *speaker, t)
			if r.err != nil {
				fmt.Printf("       ERROR: %v\n", r.err)
				last = r
				break
			}
			runs.kagomeMs = append(runs.kagomeMs, r.kagomeMs)
			runs.queryMs = append(runs.queryMs, r.queryMs)
			runs.synthMs = append(runs.synthMs, r.synthMs)
			runs.totalMs = append(runs.totalMs, r.totalMs)
			last = r
		}

		if last.err == nil {
			last.kagomeMs = int64(avg(runs.kagomeMs))
			last.queryMs = int64(avg(runs.queryMs))
			last.synthMs = int64(avg(runs.synthMs))
			last.totalMs = int64(avg(runs.totalMs))
			all.kagomeMs = append(all.kagomeMs, runs.kagomeMs...)
			all.queryMs = append(all.queryMs, runs.queryMs...)
			all.synthMs = append(all.synthMs, runs.synthMs...)
			all.totalMs = append(all.totalMs, runs.totalMs...)
		}
		results = append(results, last)
	}

	// Results table.
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Chars\tKagome(ms)\tQuery(ms)\tSynth(ms)\tTotal(ms)\tChunks\tWAV(KB)\tAudio(ms)\tText")
	fmt.Fprintln(w, "-----\t----------\t---------\t---------\t---------\t------\t-------\t---------\t----")
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(w, "%d\tERR\tERR\tERR\tERR\t-\t-\t-\t%s\n",
				r.charCount, truncate(r.text, 40))
		} else {
			fmt.Fprintf(w, "%d\t%d\t%d\t%d\t%d\t%d\t%.1f\t%d\t%s\n",
				r.charCount,
				r.kagomeMs,
				r.queryMs,
				r.synthMs,
				r.totalMs,
				r.chunkCount,
				float64(r.wavSizeBytes)/1024,
				r.audioDurationMs,
				truncate(r.text, 40),
			)
		}
	}
	w.Flush()

	// Chunk breakdown (optional).
	if *showChunks {
		fmt.Println()
		fmt.Println("── Clause chunks ───────────────────────────────────────────────────")
		for _, r := range results {
			if r.err != nil {
				continue
			}
			if r.chunkCount == 1 {
				fmt.Printf("  [1]  %s\n", r.text)
			} else {
				fmt.Printf("  [%d chunks] %s\n", r.chunkCount, truncate(r.text, 50))
				for j, c := range r.chunks {
					fmt.Printf("       %d. %s\n", j+1, c)
				}
			}
		}
	}

	// Summary stats.
	if len(all.totalMs) > 0 {
		fmt.Println()
		fmt.Println("── Summary ──────────────────────────────────────────────────────")
		fmt.Printf("%-12s  %8s  %8s  %8s\n", "", "avg(ms)", "med(ms)", "max(ms)")
		fmt.Printf("%-12s  %8.0f  %8d  %8d\n", "Kagome",
			avg(all.kagomeMs), median(all.kagomeMs), maxVal(all.kagomeMs))
		fmt.Printf("%-12s  %8.0f  %8d  %8d\n", "Query",
			avg(all.queryMs), median(all.queryMs), maxVal(all.queryMs))
		fmt.Printf("%-12s  %8.0f  %8d  %8d\n", "Synth",
			avg(all.synthMs), median(all.synthMs), maxVal(all.synthMs))
		fmt.Printf("%-12s  %8.0f  %8d  %8d\n", "Total (chunk 1)",
			avg(all.totalMs), median(all.totalMs), maxVal(all.totalMs))
		fmt.Println("─────────────────────────────────────────────────────────────────")
		fmt.Println("Note: Query+Synth times are for the FIRST chunk only.")
		fmt.Println("      With pipelining, subsequent chunks overlap with playback.")
	}
}

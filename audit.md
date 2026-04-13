prediction:
    backend will be very clean and boring
    frontend will have some messy css + not super pretty state mangagement code

routes.go

- api/endpoint handler: April 12th 


routes_words.go

- found some code paths that sill wrote audio to files
    - codex audited all backend and frontend code for reading/writing audio files and removed it all
        
routes_tutor.go (fine)

routes_stories.go 

- apiDeleteStoryNotedWord refetches whole story just to remove a single noted word
    maybe necessary with current data design?

- removed one tiny unused function storySentenceText


dict.go
dict_lookup.go

- the dicionary db preserves original dictionary structure when we just want word info per word
    - so currently lookups require a lot of work to get the word info when it should just be stored in the original compressed db directly

```
Currently, retrieving word info from the dictionary requires quite a lot of work. I'm worried about the performance when looking up word info for many words. Is there something we could do to make this faster and simpler?
```

```
13 previous messages
Yes. The biggest win here is to stop reconstructing dictionary answers at read time.

Right now one lookupDictionaryWord() call fans out into a lot of work:

candidate search across both jmdict_kanji and jmdict_kana in src/dict_lookup.go (line 62)
another query for the chosen reading in src/dict_lookup.go (line 177)
another query for senses, plus JSON decoding for each row, in src/dict_lookup.go (line 221)
an N+1 gloss lookup loop in src/dict_lookup.go (line 283)
two more queries per kanji character in src/dict_lookup.go (line 325)
and every app lookup goes through that path from src/db_words.go (line 206)
On top of that, the shared dictionary handle is forced to one SQLite connection in src/dict.go (line 124), so bulk lookups are effectively serialized.

My recommendation is:

Precompute a flattened lookup table in jdict.db.
Each row would already contain the runtime answer for one lookup text:
lookup_text
reading
part_of_speech
meaning
glosses_json
kanji_json
score
Then runtime becomes basically:

SELECT reading, part_of_speech, meaning, glosses_json, kanji_json
FROM dict_word_lookup
WHERE lookup_text = ?
ORDER BY score DESC
LIMIT 1;
That would make lookups both faster and much simpler. You’d be moving the “figure out the best candidate / filter senses / collect glosses / build kanji info” logic into the dictionary build step, which only runs once.

Add a tiny in-process memoization cache on top.
Even with a flattened table, repeated words should just hit map[string]*dictionaryWordInfo in memory.

If you want an intermediate step before a schema change:

batch lookup words with IN (...) instead of one word at a time
collapse the gloss N+1 into a single query
optionally allow more than one read-only SQLite connection
But those are incremental. The real simplification is denormalizing the dictionary into a runtime-oriented table.

So the short answer is: yes, and the cleanest approach is to treat jdict.db as a build artifact optimized for app lookups, not as a normalized source dataset queried live.

If you want, I can implement the first stage of this and add a dict_word_lookup table plus a much simpler Go lookup path.
```

morphology.go (fine)

images.go (fine)

ai.go 
ai_openai.go
...

- arguably should create AIProvider interface, but instead just uses switches on strings for handful of AI operations

wordlists.go

db_schema.go (fine)


FRONTEND

welcome page (fine)






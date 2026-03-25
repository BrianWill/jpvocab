# TODO

This file is random notes of possible features and bug fixes.

- Each word has its current total drill count and also its target drill count. By default, words where total drill count matches or exceeds target drill count are not pulled from lexicon into the drill pool. When the user re-ups a word that hits its drill count, the target is simply increased, e.g. current total count is 10 and target is 10, so setting the target to 20 means the word will be drilled another 10 times. 


- update / add / remove tests



- lexicon page

- drill page
    - button and hotkey to play the example sentence if audio is available

- maybe AI could create / find images relevant for each word
    - maybe just for nouns and verbs but not for other parts of speech

- use turso to sync db
- make a mobile app (sync through turso)

- EXPERIMENT: when finished with the project, have claude generate a claude.md file with details about the program that could be used to recreate the program from scratch in a new project 
    - make sure to embed structuring principles, like keep routes and db functions in separate files
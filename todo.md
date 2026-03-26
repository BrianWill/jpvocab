# TODO

This file is random notes of possible features and bug fixes.


- words in lexicon page list shift layout by a couple pixels when hovering (the edit / delete buttons should be visibility hidden, not display none)

- when adding words that already are in lexicon, present the remaining target count and present - + buttons to tweak the count


- when adding words, option to ask AI to give a different definition or a example sentence / translation
    - do this in the add words modal?
    - or only do this in the word edit modal?
        - presented with prior info and all subsequent re-rolls, then user picks the winner to keep

- generate audio for the words
    - pulldown options: either no audio, browser tts, or voicevox
        - selecting voicevox triggers error message if the voicevox server is not found
            - give detailed message explaining what user must do, including expected port number
    - drill page: button and hotkey to play the example sentence if audio is available
        - second and third sets of hotkeys to play at slower speeds
    - lexicon page: button to play word audio / sentence audio

- AI images
    - generate or find relevant images to associate with words
    - maybe just for nouns and verbs but not for other parts of speech
    - favor certain pics within certain aspect ratio ranges
    - rescale to target res
        - images that are too low res to be effectively upscaled will be rejected

- use turso to sync db
- make a mobile app (sync through turso)

- EXPERIMENT: when finished with the project, have claude generate a claude.md file with details about the program that could be used to recreate the program from scratch in a new project 
    - make sure to embed structuring principles, like keep routes and db functions in separate files
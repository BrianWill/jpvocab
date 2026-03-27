# TODO

This file is random notes of possible features and bug fixes.


- words in lexicon page list shift layout by a couple pixels when hovering (the edit / delete buttons should be visibility hidden, not display none)

- when adding words that already are in lexicon, present the remaining target count and present - + buttons to tweak the count

- when adding words, user should be able to enter any Japanese text to add all words 
    - filters out particles, conjunctions, and other small parts of speech
    - no longer required to separate words by newlines or whitespace
        - instead just tries to analyze the text for all discrete words  

- translate stories and generate audio via voicevox
    - can voicevox generate a single audio file but give timemarks for each line?
    - e.g. given chapter of novel, produce audio
    - experiment with openAI audio generation. Might be better quality than voicevox

- for word add, generate auto-fill menu should be very similar to the edit generate menu
    - no checkbox when entering words: will just generate in the modal after the words have been added
    - does the box list every generation attempt? user can generate multiple times and pick the best for all? 
        - awkward because you ideally can generate / pick for each word
        - 

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
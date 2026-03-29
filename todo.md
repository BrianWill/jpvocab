# TODO

This file is random notes of possible features and bug fixes.


- support other AI providers
    - Google
    - GLM?
    - who else? ask AI

- settings menu
    - gear next to nav links in header
    - saved in db
    - default drill size preference
    - preferred AI provider/model
    - preferred male and female voices

- enable wails in the prototype
    - once ready, we'll move the prototype project to main dir

- translate stories and generate audio via voicevox
    - can voicevox generate a single audio file but give timemarks for each line?
        - apparently yes https://gemini.google.com/app/64285ccfa674d709
    - e.g. given chapter of novel, produce audio
    - experiment with openAI audio generation. Might be better quality than voicevox

- when generating word info, what if the AI keeps returning the same bad results
    - Maybe need a text box so the user can add comments for the AI?
    - or explicitly ask the AI for multiple different choices and let the user pick?
        - these would be displayed indented below the word and its current info

- generate audio for the words
    - pulldown options: either no audio, browser tts, or voicevox
        - selecting voicevox triggers error message if the voicevox server is not found
            - give detailed message explaining what user must do, including expected port number
    - drill page: button and hotkey to play the example sentence if audio is available
        - second and third sets of hotkeys to play at slower speeds
    - lexicon page: button to play word audio / sentence audio

- speaking tutor chatbot
    - user types, chatbot responds with generated audio (voicevox?)

- AI images
    - generate or find relevant images to associate with words
    - maybe just for nouns and verbs but not for other parts of speech
    - favor certain pics within certain aspect ratio ranges
    - rescale to target res
        - images that are too low res to be effectively upscaled will be rejected

- use turso to sync db
- make a mobile app (sync through turso)

- EXPERIMENT: when finished with the project, have claude generate a claude.md file with details about the program that could be used to recreate the program from scratch in a new project 
    - express as a list of features / requirements, not as a spec for implementation
        - i.e. don't prescribe code structure or specifics of UI layout
        - only specify broad strokes of tech stack
    - generate a checklist plan, then have it follow the plan, step-by-step
        - it is allowed to revise the plan if it desires
        - but don't test or offer detailed feedback between steps
        - user is mainly there to tell it to keep going after each step
            - "are you done? OK revise your tasklist, I'll clear context, and you will resume
        - if stuck, elevate effort level? or do whole experiment on effort high?
        - try once with sonnet, and once with opus
    - make sure to embed structuring principles, like keep routes and db functions in separate files
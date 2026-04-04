# TODO

This file is random notes of possible features and bug fixes.

- pitch accent info on readings

- clicking the image placeholder or thumbnail shoudl open a file dialog to upload an image file

- stories page
    - each sentence of Japanese text has map of positions to words, e.g. { pos: 35, word: "言葉" } indicates the word which begins at character 35 (is index of character a reliable way to index into a Japanese sentence?)
    - maybe each sentence is actually just stored as a list of words, each with its baseform and the actually displayed form in the text
        (so when displayed, the sentence is constructed from the display forms)
    - translate stories and generate audio via voicevox
        - can voicevox generate a single audio file but give timemarks for each line?
            - apparently yes https://gemini.google.com/app/64285ccfa674d709
        - e.g. given chapter of novel, produce audio
        - experiment with openAI audio generation. Might be better quality than voicevox
    
    - scan text to find most frequently occuring words in text that are candidates for the lexicon

- play with visual styles
    - use simple organic textures instead of solid background colors
        - e.g. wood, paper, leaves
        - what textures are uniquely Japanese?
    - noise background textures?
    - need mascot character(s)?
        - marble animals? 
    - shadcn

- enable wails in the prototype
    - once ready, we'll move the prototype project to main dir



- grammatical analysis
    - sentence breakdown
    - isolate, classify, and explain phrases

- translation exercise:
    - use words from users lexicon     
    - present sentence in Japanese to translate to English
    - present English trnaslation to translate to Japanese

- speaking tutor chatbot
    - user types, chatbot responds with generated audio (voicevox?)
    - instruct bot to use words that are in user's active lexicon
        - these encounters could be tallied as correct drills? 


- make a mobile app (sync through turso)

- some way to sync db state
    - what if just one version of db in google drive for each machine
        - manually sync between them with our own logic instead of relying on rclone or google drive 
        - user specifies paths in settings (or they are all expected to live in one directory of google drive path)
        - for streaming file mode, is reads / writes to sqldb going to be too slow?

    - use turso?
        - but optional...(annoying if users would be required to sign up for an account)
    - syncthing?
    - litestream
    - rqlite
    - litefs
    - main use case for now: sync between desktop and laptop
    - maybe simpler solution: just always sync db to a server instead of p2p?
        - p2p requires both systems on and connected
    - maybe just use rcloud and google drive?
        - create script to help you setup rcloud on mac or windows?


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

# TODO

This file is random notes of possible features and bug fixes.

- preserve drill state in db
    - would allow user to switch away to different pages without worrying about losing their progress

- play with visual styles
    - use simple organic textures instead of solid background colors
        - e.g. wood, paper, leaves
        - what textures are uniquely Japanese?
    - noise background textures?
    - need mascot character(s)?
        - marble animals? 
    - shadcn

- the word lists should have pre-filled info (and images?) in their json
    - when any word is added, the words from the lists should be checked to see if there is already info available
        - they could also have a url to a suggested image to acquire (so we don't have to include the images in the repo)


- hovering a word in the activity page date info should show its word info (like the sidebar on the drill page)

- settings menu
    - preferred AI provider/model
        - only applicable if the user has multiple available keys
    - default value for target drill count of new words

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

- generate audio for the words
    - pulldown options: either no audio, browser tts, or voicevox
        - selecting voicevox triggers error message if the voicevox server is not found
            - give detailed message explaining what user must do, including expected port number
    - drill page: button and hotkey to play the example sentence if audio is available
        - second and third sets of hotkeys to play at slower speeds
    - lexicon page: button to play word audio / sentence audio
    - in settings, pick preferred male and female voices
        - settings menu let's you play sample of the voices

- AI images
    - generate or find relevant images to associate with words
    - maybe just for nouns and verbs but not for other parts of speech
    - favor certain pics within certain aspect ratio ranges
    - rescale to target res
        - images that are too low res to be effectively upscaled will be rejected

- enable wails in the prototype
    - once ready, we'll move the prototype project to main dir

- translate stories and generate audio via voicevox
    - can voicevox generate a single audio file but give timemarks for each line?
        - apparently yes https://gemini.google.com/app/64285ccfa674d709
    - e.g. given chapter of novel, produce audio
    - experiment with openAI audio generation. Might be better quality than voicevox
 
- translation exercise:
    - use words from users lexicon     
    - present sentence in Japanese to translate to English
    - present English trnaslation to translate to Japanese

- grammatical analysis
    - sentence breakdown
    - isolate, classify, and explain phrases

- speaking tutor chatbot
    - user types, chatbot responds with generated audio (voicevox?)
    - instruct bot to use words that are in user's active lexicon
        - these encounters could be tallied as correct drills? 


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

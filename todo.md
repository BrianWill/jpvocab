# TODO

This file is random notes of possible features and bug fixes.

- story noted words
    - scan text to auto suggest noted words from the frequently occuring unique words in the story

- story list
    - add a date last viewed to optionally sort by?

- story page
    - option to filter out presenting translations/info of particles and other very common words

- enable wails in the prototype
    - once ready, we'll move the prototype project to main dir


- write the README
    - need setup instructions
        - add these instructions or link them on the welcome page
    - explain drilling theory / workflow

- story translation tooltip
    - reading should include pitch info and separated by kanji (like on lexicon or in drill)
    - also display kanji info? per kanji info would allow for consistent display 
        - make it match the word info tooltip of the drill page? (even include image?)
        
- proper error messages for failed API requests
    - what if user runs out of tokens? or other API failures




- experiment with openAI audio generation. Might be better quality than voicevox

- play with visual styles
    - use simple organic textures instead of solid background colors
        - e.g. wood, paper, leaves
        - what textures are uniquely Japanese?
    - noise background textures?
    - need mascot character(s)?
        - marble animals? 
    - shadcn


- grammatical analysis of story sentences
    - click sentence to open analysis modal 
        - what is hotkey? how to distinguish from click-to-play? ctrl-click? hover popup next to sentence?
        - uses AI to give breakdown of phrases / clauses
        - has chat window so user can ask about points of grammar (continues context of the sentence breakdown)
        - analysis and chat session of each sentence is preserved in db
            - maybe option to clear the conversation?
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

# TODO

This file is random notes of possible features and bug fixes.

- when you answer word, maybe there should be brief reveal of answer before moving to next word?
    - especially when you answer don't know
    - or maybe just delay reveal and audio play of next word
    - or maybe show answer in main area, then user has to hit space to proceed
    - should answering repeat playback of the word?
    - space could also be hotkey that advances to next word without answering the word
        - what happens to unanswered words when you get to next round?

- separate generation of example sentences from generation of word info

- on story import, can we detect if a sentence is Japanese or English (mixed cases default to Japanese)?

- for repurposing English content, may be best to translate on paragraph or multi-graph level, then split into sentences?
    - would require translating back

- video stories with subtitles
    first import subtitle files

    either local video file or embedded youtube 

- add more common Japanese words to the word insertion blacklist?
    already filtering based on POS, but some common things sneak through, like　です

- story       
    - word info retrieval and presentation still needs work
        filter out more noise words
            have the server mark words that should show work info in the json
                frontend should only request wordinfo for words that might have it
                frontend shows nothing for words that cannot have word info
                    if no translation, teh hover tooltip says "no translation available"
    

- memory usage
    nearly 300mb
        mainly the dict?

- the wails zoom override is interferring with layout
    not full equivalent of browser zoom

- activity page
    - stats on number of tracked words in lexicon vs untracked

- mic STT : why only last a few seconds? what causes it to stop?
    - should user have to hit button to stop STT? also stop STT when user submits the message?

- drill
    - presentation of last word needs some reconsideration
        ideally shouldn't move around
    - modal to edit current word
        - sidebar, edit button appears when hovered

- stories:
    - support English stories that get translated into Japanese (sentence by sentence)
        in this setup, going for decent natural translations by chunk rather than by sentence?
            if so, need to also translate back to English sentence-by-sentence to get gloss of each Japanese sentence
    
- tutor:
    - ability to hold separate chats at same time in separate tabs
        - wouldn't work in wails, but that's OK?
    - prompts should all be tolerant of swapping between English and Japanese. If English, don't provide critque of their language, but otherwise just procede whether they answer in Japanese or English.
    - single click on words in bot messages to add them to the dictionary
    - maybe play the AI's correction? use different voice?
    - if AI provides a correction, your last message is faded and teh correction placed below it with new color
    - should warn users somehow about conversation getting too long (context collapse)
    - button to suggest topics, sentences? 
        maybe a separate AI request? 
        pull from news? 
        pull from stock set of topics?
            try to fit topics with your vocabulary?
    - button / hotkey to have the AI write a message for the user
    - games:
        - describe what is in the picture
        - modes where the user only expected to answer in English
    - translation exercise:
        - use words from users lexicon     
        - present sentence in Japanese to translate to English
        - present English trnaslation to translate to Japanese
    - how to integrate lexicon? 
        generate topics / questions from a random set of recently drilled words? or from active words?
        - instruct bot to use words that are in user's active lexicon
        - these encounters could be tallied as correct drills? 
    - option to rewind chat? undo last prompt and response?
    - in chat, highlight words in lexicon, different color for words that are active? 
        - when user responds to bot message that contains active word, increment that drill count?
    - bot that provides grammatical analysis of sentences
        user can ask questions in English / Japanese or mix
            - what is hotkey? how to distinguish from click-to-play? ctrl-click? hover popup next to sentence?
            - uses AI to give breakdown of phrases / clauses
            - has chat window so user can ask about points of grammar (continues context of the sentence breakdown)
            - analysis and chat session of each sentence is preserved in db
                - maybe option to clear the conversation?
        - sentence breakdown
        - isolate, classify, and explain phrases
    - typing trainer bot
        - this could be just another chat bot
            seems wasteful maybe to use tokens, but the tokens should be pretty cheap


- wails
    - double scrollbar appears on start of wails
    - on mac, cmd+-/+ does weird zoom behavior
        (but ctrl+scroll and ctrl+-/+ is fine)
    - custom scroll behaviour breaks page layout

- lexicon
    - slow to render with thousands of words
        - slow to scroll? slow toload? slow to sort? 
        - slow on client or backend?
        - add pagination?
        - add search for lexicon list?

- story list
    - add a date last viewed to optionally sort by?

- video stories
    - play local files
        sync with subtitles
    - embed youtube player?

- story page
    - scan text to auto suggest noted words from the frequently occuring unique words in the story
        - maybe just auto add high-frequency words to noted words, 
            e.g. add all words in story tha occur more than N times
            - filter out proper names, particles, conjunctions, etc.

- increase the base size of fonts i.e. effective base zoom level

- README
    - explain drilling theory / workflow
 
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

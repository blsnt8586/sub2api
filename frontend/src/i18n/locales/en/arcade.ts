import tower from './tokenTower'

export default {
  ocean: {
    campaign: {
      round: 'Stage {n} / {total}', chapter: 'Chapter {n} / {total}',
      next: 'Next challenge', remaining: 'Time left',
      clearedHint: 'Stage cleared. Recover one heart on the next stage; complete all ten chapters to win.',
      time: '{boss} · {n} seconds', boss: 'Hunter duel', hunt: 'Regional hunt',
      final: 'Hunt complete · Grow fully for this chapter’s hunter duel',
      savedHint: 'Local progress restored. Start to replay this stage from its beginning.',
    },
    voyage: {
      eat: 'Stage {stage}/{totalRounds} · {direction}{zone} · Eat {n}/{total}',
      dash: 'Stage {stage}/{totalRounds} · {direction}{zone} · Dash catches {n}/{total}',
      golden: 'Stage {stage}/{totalRounds} · {direction}{zone} · Golden prey {n}/{total}',
      final: 'Three regions cleared · Grow fully for the hunter duel',
      cleared: 'Leg cleared! Bonus score, one heart and full dash stamina',
      weather: {
        bloom: 'Feeding tide · Fresh schools arrive · {seconds}s',
        current: 'Following current · Faster swimming and stamina recovery · {seconds}s',
        golden: 'Golden tide · More golden prey · {seconds}s',
      },
    },
    exploration: {
      zones: {
        reef: 'SUNLIT REEF',
        twilight: 'TWILIGHT ZONE',
        abyss: 'BLUE ABYSS',
      },
      relic: 'DEEP RELIC',
      migration: 'GIANT MIGRATION',
      gentle: 'GENTLE GIANT · SAFE TO APPROACH',
      progress: 'Species {found}/23 · Relics {relics}/3',
      collection: '{n}/23 discovered this run · Expand field guide',
      seen: 'SEEN',
      escort: 'Follow the giant and gather crystals · {n}/6 · {seconds}s',
      discovery: 'First encounter · {name}',
      diveHint: 'Swim down into the deep · Blue arrows lead to relics',
      intro:
        'Descend from the reef into the abyss. Discover glowing schools and gentle whales, gather crystals along migration routes, and find three relics. Keep growing after the final form. Three hits end the expedition.',
      messages: {
        none: 'Keep exploring the ocean',
        zone: 'A new water layer · Look for different species and giants',
        relic: 'Relic found! +600 points and full dash stamina',
        relicComplete: 'All three relics! +1800 points and one life restored',
        migration: 'A giant is passing! Gather six crystals within 18 seconds',
        migrationClear: 'Migration complete! +700 points and one life restored',
      },
    },
    ecology: {
      cooldown: 'Frenzy cooling down · {n}s',
      finale: 'FINALE · HAMMERHEAD',
      guideTitle: 'Ocean field guide · 23 creatures',
      guideHint: 'Know your prey. Choose your next bite · Expand',
      spines: 'WAIT FOR SPINES',
      venom: 'VENOM · FRENZY ONLY',
      ambush: 'AMBUSH · SWIM SIDEWAYS',
      pressure: {
        '0': 'ALERT I · SHALLOWS',
        '1': 'ALERT II · HUNTING GROUND',
        '2': 'ALERT III · DANGER ZONE',
      },
      names: {
        clownfish: 'Coral clownfish',
        butterflyfish: 'Golden butterflyfish',
        sardine: 'Silver sardine',
        pufferfish: 'Spiny pufferfish',
        lionfish: 'Scarlet lionfish',
        barracuda: 'Silvertooth barracuda',
        hammerhead: 'Hammerhead hunter',
        anglerfish: 'Lantern anglerfish',
        lanternfish: 'Starlight lanternfish',
        hatchetfish: 'Silver hatchetfish',
        viperfish: 'Abyss viperfish',
        'gulper-eel': 'Giant gulper eel',
        oarfish: 'Ruby-crested oarfish',
        manta: 'Blue-wing manta',
        'giant-squid': 'Crimson giant squid',
        'whale-shark': 'Starlit whale shark',
        'blue-whale': 'Ancient blue whale',
      },
      hints: {
        minnow: 'Small sunlit-reef prey, ideal for your first bites.',
        bluegill:
          'Schooling sunfish, also the main target of golden-school events.',
        perch: 'A shallow-water predator. Grow larger to turn the tables.',
        koi: 'Swims around the reef and twilight waters. Check its size before chasing.',
        tuna: 'A fast open-water predator. Keep space to turn or dash.',
        moonfish: 'A twilight swimmer and your final evolution form.',
        clownfish:
          'Orange reef schools reward a well-planned dash through the group.',
        butterflyfish:
          'Golden fins are easy to spot; sizes vary, so choose your target.',
        sardine:
          'Flees faster when startled. Catch one to restore 16 stamina and keep dashing.',
        pufferfish:
          'Glows before inflating. Its spines block feeding even in frenzy. Eat it when deflated and small enough for double points.',
        lionfish:
          'Venomous even when small. Dodge its fins; frenzy lets you eat it within the size limit for double points.',
        barracuda:
          'Telegraphs, locks its aim, then charges straight. Swim sideways; its sprint cannot turn.',
        anglerfish:
          'Patrols quietly, then accelerates when you get close. Avoid following its lure into a corner.',
        hammerhead:
          'A large twilight predator. The challenge hunter charges: dodge its lane, then strike its tail during recovery. Three final strikes win.',
        lanternfish:
          'Glowing prey in twilight and abyss waters. Schools offer a route through the dark.',
        hatchetfish:
          'Its silver belly is easy to spot. Small prey still thrives in the deep.',
        viperfish:
          'A deep-water ambusher. Its telegraphed charge locks direction; swim sideways.',
        'gulper-eel':
          'A deep predator with a huge mouth. Grow larger or counter it during frenzy.',
        oarfish:
          'A giant silver ribbon in deep waters. Harmless; approach to discover it.',
        manta:
          'A gentle reef giant. Sometimes leads a timed crystal-gathering route.',
        'giant-squid':
          'A large abyss predator with sprawling tentacles. Keep clear until the right moment.',
        'whale-shark':
          'A spotted twilight giant. It cannot hurt you or be eaten.',
        'blue-whale':
          'An enormous gentle silhouette in the abyss. Follow its crystal route when migrating.',
      },
    },
    challenge: {
      chooseMode: 'Choose a mode',
      challenge: 'Hunter challenge',
      relaxed: 'Relaxed feeding',
      explore: 'Deep expedition',
      exploreHint: 'Giant migrations · Relics · Endless growth',
      challengeHint: 'Special species · Rising pressure · Hammerhead finale',
      relaxedHint: 'Grow freely. Evolve to win.',
      intro:
        'One continuous 60-stage challenge across ten chapters. Complete timed eating, dash and golden-prey objectives; face a hunter every six stages. All sixty stages are required to win, with local checkpoints between visits.',
      hunter: 'HAMMERHEAD HUNTER',
      tailTarget: 'TAIL WEAK POINT',
      shoal: 'GOLDEN SCHOOL',
      aim: 'Hunter aiming! Leave the illuminated charge lane',
      charge: 'Charge! Swim sideways or dash through',
      recover: 'Hunter exhausted! Dash into the glowing tail',
      objective: 'Eat to evolve. Watch for golden schools and the hunter.',
      finalObjective: 'Finale · Dodge, then dash at the tail',
      bossHealth: '{n} tail strikes remaining',
      shoalProgress:
        '{seconds}s · School {n} / {total} · Clear for +500 points and one life',
      starHint:
        'One star for winning; one for finishing within 120 minutes; one for keeping 2 lives with at least 5 dodges and tail strikes combined.',
      stars: '{n} stars earned',
      runStats: '{seconds}s · {dodges} close dodges · {bites} tail strikes',
      banners: {
        shoal:
          'Golden school! Follow the gold arrow and clear it in 11s to heal',
        shoalClear: 'School cleared! +500 points and one life restored',
        hunter: 'Hunter approaching! Read its charge lane',
        tail: 'Tail strike! A perfect counterattack',
        pressure: 'Reef alert rising! Predators are more active. Save a dash.',
        dodge: 'Close dodge! +120 points',
        finale:
          'Final battle! Dodge the charge, then strike its tail three times',
      },
    },
    title: 'TOKEN Feed & Grow',
    subtitle: 'One little bite. One much bigger adventure.',
    description:
      'Chase smaller fish, dodge predators, and grow. Chain bites for a frenzy, dash out of danger, and rule the reef.',
    duration: '60 stages · 10 chapters · Local checkpoints',
    score: 'SCORE',
    stage: 'Evolution {n} / 6',
    growth: 'Evolution progress',
    stageLabel:
      'Feed and Grow. Move the mouse, drag, or use WASD to swim. Space to dash, P to pause.',
    sceneLabel:
      'An illustrated coral reef. Control the fish under the small arrow, dash through schools, and turn the tables on ordinary predators during a frenzy.',
    you: 'YOU',
    eat: 'EAT',
    danger: 'DANGER',
    lives: '{n} lives remaining',
    frenzy: 'YOUR TURN TO HUNT',
    frenzyHint:
      'Frenzy overcomes predators and venom, but inflated puffers and the hunter stay dangerous.',
    feast: '{n} FISH. ONE GULP!',
    feastHint: 'Dash through a school and feast',
    chainHint: 'Chain bites. Turn the tables.',
    dash: 'Dash',
    stamina: 'Dash stamina',
    readyTitle: 'Small fish. Big appetite.',
    intro:
      'Catch a little fish. Take a bite. Grow big enough to make yesterday’s predator tomorrow’s lunch.',
    ruleEat: 'Eat small. Grow big.',
    ruleAvoid: 'Dodge the big ones.',
    ruleDash: 'Dash to chase or escape.',
    start: 'Dive in',
    retry: 'One more bite',
    resume: 'Keep swimming',
    paused: 'Take a breath.',
    pauseHint:
      'The reef and your progress are paused. Jump back in when you are ready.',
    overTitle: 'A good run. A bigger appetite.',
    clearedTitle: 'Another stage conquered.',
    wonTitle: 'The reef is yours.',
    result: '{score} points · {count} fish eaten · Best chain: {combo}',
    pointerHint: 'Move your mouse or drag your finger to swim',
    moveHint: 'Mouse / drag / WASD / left stick · Swim',
    keyHint: 'Space / E / A · Dash　P / Menu · Pause',
    current: 'YOU',
    evolutionPath: 'Six stages of evolution',
    loading: 'Diving into the reef…',
    artError: 'Reef artwork could not load',
    notices: {
      clear: 'Stage cleared. Prepare for the next challenge.',
      welcome: 'Eat smaller fish to grow. Avoid the bigger ones.',
      evolve: 'Evolved! You can now eat bigger fish.',
      hurt: 'Hit! Swim away while your shield is active.',
      frenzy: 'Feeding frenzy! Swim faster and keep eating.',
      won: 'All six forms unlocked. You rule the reef!',
      over: 'Run complete. Ready for another bite?',
    },
  },
  arcade: {
    chapter: 'Region {n} / 10 · One equipment upgrade every ten shafts',
    shaftGoal: 'Target {value} T · Crystals recovered {gems}/{required}',
    savedHint: 'Local expedition restored. Start to replay this shaft from its beginning.',
    layouts: { scattered: 'Scattered ore', veins: 'Striped veins', deep: 'Buried crystals', walls: 'Rock barriers', crystals: 'Crystal clusters' },
    gear: 'Winch {winch} · Clock {clock} · Assay {assay}',
    conditions: {
      stable: 'Stable seam · Starred ore is worth more',
      crystal: 'Crystal seam · Crystals worth 35% more',
      dense: 'Dense seam · Large ore worth 30% more, but heavier',
      loose: 'Loose seam · Rocks weigh half as much',
    },
    upgrades: {
      choose: 'Choose an upgrade · Up to three levels each',
      winch: 'Power winch', winchHint: '+22% retrieval speed per level',
      clock: 'Time battery', clockHint: '+10 seconds in later shafts per level',
      assay: 'Crystal scanner', assayHint: '+20% crystal value per level',
    },
    artworkLoading: 'Preparing the mine…',
    artworkRetry: 'Artwork unavailable. Retry',
    practice: 'PRACTICE',
    title: 'TOKEN Park',
    subtitle: 'Take a little break. Dig up something good.',
    sessionBest: 'Session best',
    gameTitle: 'Token Miner',
    level: 'Level {n}',
    mute: 'Mute sound',
    unmute: 'Enable sound',
    pause: 'Pause',
    resume: 'Keep digging',
    haul: 'YOUR HAUL',
    target: 'TARGET',
    remaining: 'TIME LEFT',
    canvasLabel:
      'Token mine. Click, press Space or Arrow Down to launch. P to pause, E to use dynamite.',
    canvasFallback: 'Your browser needs Canvas support to display the mine.',
    practicePoints: 'practice points',
    rewardNotice: 'Practice points cannot be exchanged for API credit.',
    launchHint: 'or click the mine to launch',
    discardLabel: 'Use dynamite to discard the ore; {n} remaining',
    discard: 'Dynamite',
    launch: 'Drop the claw',
    retrieving: 'Retrieving…',
    expedition: 'THE EXPEDITION',
    missionTitle: 'A little deeper.',
    missionDescription:
      'One hundred shafts across ten regions. Upgrade every ten clears and adapt to varied layouts, seams and crystal quotas.',
    targetValue: 'Target {n} T',
    total: 'Expedition total',
    fieldGuide: 'FIELD GUIDE',
    knowYourOre: 'Make every grab count.',
    ores: {
      token: 'Token ore',
      large: 'Large token',
      gem: 'Compute crystal',
      rock: 'Ordinary rock',
    },
    oreHints: {
      token: 'Light and quick to retrieve',
      large: 'More value, more weight',
      gem: 'Rare, light, and valuable',
      rock: 'Heavy, with little value',
    },
    practiceNoteTitle: 'Just a little fun, for now.',
    practiceNote:
      'Practice campaign progress is saved in this browser. Points do not grant balance or API credit. Clearing browser data removes checkpoints.',
    howTitle: 'A good haul takes timing.',
    steps: {
      '0': {
        title: 'Find your moment',
        description:
          'Watch the swinging claw. Aim for tokens and teal crystals.',
      },
      '1': {
        title: 'Drop the claw',
        description:
          'Click the mine, the button, or press Space. Caught a rock? Press E for dynamite.',
      },
      '2': {
        title: 'Bring it home',
        description:
          'Retrieve ore before the timer runs out. Reach the target to unlock the next level.',
      },
    },
    nextLevel: 'Next level',
    retry: 'Retry this level',
    playAgain: 'Start another expedition',
    start: 'Start mining',
    started: 'Level {n} started',
    discarded: 'Dynamite used; ore discarded',
    paused: 'Game paused',
    resumed: 'Game resumed',
    autoPaused: 'Game automatically paused while you were away',
    collected: 'Collected {n} points; level total {total}',
    result: 'Level haul: {n} points',
    levelNames: {
      '0': 'The shallows',
      '1': 'Crystal cavern',
      '2': 'Deep treasures',
      '3': 'Fault line',
      '4': 'Golden river',
      '5': 'Ember cavern',
      '6': 'Ancient core',
      '7': 'Earth vault',
      '8': 'Meteor field',
      '9': 'Motherlode',
    },
    overlays: {
      ready: {
        title: 'Dig up something good.',
        description:
          'Aim your claw and take your shot. Collect {target} points in {seconds} seconds to start your expedition.',
      },
      paused: {
        title: 'The mine can wait.',
        description: 'The timer is paused. Keep digging when you are ready.',
      },
      won: {
        title: 'A very good haul!',
        description:
          'Target reached. Another pocket of treasure is waiting for you.',
      },
      lost: {
        title: 'So close. Dig again.',
        description:
          'Time is up. Prioritize crystals and use dynamite when rocks slow you down.',
      },
      complete: {
        title: 'A miner to remember.',
        description:
          'All one hundred shafts cleared. That is a proper Token Miner expedition.',
      },
    },
  },
  park: {
    wordmark: 'PARK',
    subtitle: 'Pick something you love. Make a little time to play.',
    featured: 'FEATURED GAME',
    singlePlayer: 'Single player',
    minerDescription:
      'Head into a glowing crystal cavern. Aim, drop the claw, and bring your tokens home. One hundred shafts, ten mining regions, varied layouts and crystal quotas.',
    mergeDescription:
      'Start with a little 2. Find the right moves, make room, and work your way to 2048. Quietly hard to put down.',
    fishingDescription:
      'Choose a voyage, salvage treasure, dodge jellyfish and chase golden visitors.',
    action: 'Action',
    adventure: 'Adventure',
    puzzle: 'Puzzle',
    relax: 'Relaxing',
    recent: 'Recently played',
    all: 'All games',
    minerTime: '100 shafts · 10 regions · Local checkpoints',
    mergeTime: 'No time limit',
    fishingTime: 'Open-ended · Autosave',
    controls: 'Keys / touch / controller',
    play: 'Play now',
    browse: 'Browse library',
    collection: 'THE COLLECTION',
    library: 'Your library',
    filter: 'Game categories',
    best: 'Best score',
    session: 'This play session',
    rounds: 'Recorded rounds',
    wins: 'Challenges cleared',
    practiceScore: 'Practice points',
    lastPlayed: 'Last played',
    sessionEmpty: 'Your story starts here',
    firstAdventure: 'Find your first adventure',
    selectPlay: 'Select / play',
    navigate: 'Browse games',
    consoleMode: 'Console mode',
    exitConsole: 'Exit console mode',
    controllerConnected: 'Controller connected',
    emptyTitle: 'Your adventures are ahead.',
    emptyDescription: 'Play a round and your journey will appear here.',
    practiceNote: 'Practice mode · Session records · No API credit rewards',
    sessionNote: 'Records stay in this browser session and reset on refresh.',
    back: 'Back to library',
  },
  fishing: {
    returning: 'Returning…',
    magnetRig: 'Salvage magnet · No fish bites',
    magnetHint:
      'Fish ignore the magnet. Switch back to bait anytime with an empty hook. Contracts track rewards without restricting fishing.',
    salvageCast: 'Dive for treasure',
    rigSelect: 'Choose your activity',
    salvageMode: 'Salvage magnet',
    fishingMode: 'Baited hook',

    routeSelect: 'Choose your voyage',
    routeLocked: 'Level 2 line',
    routes: {
      reef: {
        name: 'CORAL SHOALS',
        hint: 'Calm water · Treasure practice',
      },
      wreck: {
        name: 'WRECK PASSAGE',
        hint: 'Cross currents · More jellyfish',
      },
      trench: {
        name: 'BLUE TRENCH',
        hint: 'Strong currents · Higher value',
      },
    },
    integrity: 'Hook integrity {health} of 3',
    combo: 'Streak {count}',
    cargo: '{count} chests · {value} T aboard',
    goldenTide: 'Golden visitor · {seconds}s',
    goldenCatch:
      'A golden variant! Its base sale value is three times the regular fish.',
    dash: 'Dash',
    activeContract: 'YOUR VOYAGE OBJECTIVE',
    contractProgress: 'Harbor contract progress',
    contractComplete: 'Bounty received! Take another contract once ashore.',
    contractIntro:
      'Choose your objective. Bounties pay automatically; contracts can be repeated. Switching resets current contract progress.',
    acceptContract: 'Accept contract',
    inProgress: 'In progress',
    treasureCount: '{count} / 21 salvaged',
    contracts: {
      salvage: {
        name: 'Treasure salvager',
        hint: 'Retrieve 2 chests safely. Dashing sweeps up nearby treasure.',
      },
      hunt: {
        name: 'Fresh fish delivery',
        hint: 'Land 3 fish of any species. Sell or keep them as bait.',
      },
      streak: {
        name: 'Catch streak master',
        hint: 'Land 3 consecutive fish after accepting. Empty casts or broken lines end the streak.',
      },
    },
    title: 'TOKEN Fishing',
    subtitle: 'Chase treasure, dodge jellyfish, and catch a golden tide.',
    saved: 'Saved in this browser',
    saveError: 'Storage unavailable · Play this session',
    location: 'MINTWATER COVE',
    region: 'Ocean discovery · Chapter {level}',
    fullscreen: 'Immersive mode',
    exitFullscreen: 'Exit immersive mode',
    resume: 'Back to the cove',
    paused: 'The ocean can wait.',
    pauseHint:
      'Fish, hook and line are paused. Continue whenever you are ready.',
    stageLabel:
      'Mintwater Cove. Tap to aim and cast, drag to steer. Space toggles reel and sink, arrow keys steer, P pauses.',
    sceneLabel:
      'A painted 2D ocean cutaway with a cat angler and six animated fish species that swim, chase bait and hunt.',
    depth: 'Hook depth',
    lockedDepth: 'Upgrade your line to explore deeper',
    loading: 'Opening your little cove…',
    artError: 'The cove artwork could not load. Please retry.',
    retryArt: 'Reload artwork',
    newSpecies: 'FIRST DISCOVERY',
    catchTitle: 'A LOVELY CATCH',
    catchChoice: 'Sell for better gear, or keep as bait for a bigger catch?',
    sell: 'Sell · + {value} T',
    keepBait: 'Keep as {size} bait',
    lineStatus: 'ON THE LINE',
    left: 'Steer aim or hook left',
    right: 'Steer aim or hook right',
    tension: 'Tension',
    wallet: 'YOUR LITTLE FORTUNE',
    gameCurrency: 'In-game currency',
    harbor: 'Harbor panel',
    tabs: {
      gear: 'Tackle',
      collection: 'Journal',
      quests: 'Bounties',
    },
    gearIntro:
      'Sell a few fish for a longer line. The big fish below will also need a stronger rod.',
    gear: {
      depth: 'Deepwater line',
      strength: 'Resilient rod',
    },
    depthInfo: 'Explore down to {depth} m',
    strengthInfo: 'Safely land {size} fish',
    maxLevel: 'Fully upgraded',
    upgrade: 'Upgrade · {price} T',
    gearHint: {
      depth: 'Reach new layers of water and discover the fish living there.',
      strength:
        'Big fish can break a weak line. Let it sink to ease rising tension.',
    },
    baitInfo:
      'Basic bait is unlimited. Keep a catch as bait: small fish lure medium fish; medium fish lure large fish.',
    baits: {
      '0': 'Basic bait · unlimited',
      '1': 'Small live bait · lures medium fish',
      '2': 'Medium live bait · lures large fish',
    },
    sizes: {
      '1': 'small',
      '2': 'medium',
      '3': 'large',
    },
    phases: {
      idle: 'Pick a spot. Find your fish.',
      casting: 'A little arc of summer',
      sinking: 'Letting the hook sink',
      reeling: 'Reeling it home',
      landed: 'A lovely catch!',
    },
    actions: {
      idle: 'Cast away',
      casting: 'Casting…',
      sinking: 'Reel in',
      reeling: 'Let it sink',
      landed: 'Choose your catch',
    },
    tips: {
      idle: 'Tap to cast · Switch treasure runs / fish hunts in the harbor',
      casting: 'Splash! Your underwater adventure begins',
      sinking: 'Drag to steer · E to dash / sweep treasure · ↑ reel',
      reeling: 'Bring your cargo home · ↓ sink · E dodge',
    },
    notices: {
      fishingRig:
        'Baited hook ready. Fish can bite. Your contract progress is preserved.',
      salvageRig:
        'Magnet equipped. Fish ignore it; switch back anytime while the hook is empty.',
      treasure:
        'Chest hooked! Bring it to the surface to bank it. A broken line loses all cargo on this cast.',
      treasureBanked:
        'Treasure safely ashore! Coins banked and salvage contract updated.',
      shocked:
        'Jellyfish sting! One integrity point lost. Dodge or press E to dash through.',
      dash: 'Dash! Brief protection, faster movement, and a sweep for nearby chests.',
      routeChanged:
        'New route selected. Currents, jellyfish and chest positions change your path.',
      goldenTide:
        'Golden visitor! Find the glowing bluegill within 25 seconds for triple base value.',
      goldenCaught:
        'Golden catch ashore! The rare variant and streak bonus are included in its value.',
      contractSelected:
        'Harbor contract accepted. Choose your route and complete the objective for an automatic bounty.',
      contractDone:
        'Contract complete! Bounty paid. Accept another contract once ashore.',
      welcomeBack:
        'Welcome back to the cove. Your gear is ready for the next discovery.',
      welcome:
        'Baited hook ready. Tap to fish. Switch to a magnet for treasure; contracts never block fish bites.',
      cast: 'Your hook sinks naturally. Drag to steer and help fish discover the bait.',
      sink: 'Take it slowly. Koi prefer still bait; releasing the line also eases tension.',
      reel: 'Bringing the hook home. Still looking? Let it sink to keep exploring.',
      hooked:
        'Fish on! Reeling automatically. Let it sink to lure a larger predator with your catch.',
      tooHeavy:
        'Too heavy for this rod! Release tension or return with a stronger rod.',
      predator:
        'A bigger fish swallowed your catch! More valuable, but heavier on the line.',
      broken:
        'The line snapped and the bait is gone. Upgrade your rod for heavier fish.',
      empty:
        'The hook is home. Choose another spot and try the moving schools.',
      caught:
        'Safely ashore and added to your journal. Sell it, or fish for something bigger?',
      sold: 'Coins received. Visit the tackle shop or cast again.',
      baitReady:
        'Live bait equipped! Look for larger fish; small fish will ignore this bait.',
      upgraded: 'New gear, new waters. Your next discovery is waiting.',
    },
    collectionIntro:
      'Every species has its own habits. Bring one ashore to mark it in your journal.',
    fish: {
      minnow: 'Silverleaf minnow',
      bluegill: 'Sunburst bluegill',
      perch: 'Jade perch',
      koi: 'Coral koi',
      tuna: 'Ocean bluefin',
      moonfish: 'Mooncrest',
    },
    fishHints: {
      minnow: 'Schools in the shallows. Loves basic bait.',
      bluegill: 'A mellow neighbor that likes basic bait.',
      perch: 'Hunts small fish, including those on your hook.',
      koi: 'Eats small fish. Fast-moving hooks scare it away.',
      tuna: 'Hunts medium fish. Needs a strong rod.',
      moonfish: 'A deepwater secret that eats medium fish.',
    },
    questIntro:
      'Little goals, taken slowly. Rewards go straight into your wallet.',
    quests: {
      first: {
        title: 'A little fortune',
        hint: 'Bring 3 fish safely ashore.',
      },
      hunter: {
        title: 'Big fish, little fish',
        hint: 'Use small fish as bait to land a perch or koi.',
      },
      deep: {
        title: 'A letter from the deep',
        hint: 'Explore below and land a bluefin or mooncrest.',
      },
    },
    rewardClaimed: 'Reward received',
    totalCaught: '{count} fish landed',
    noTimer: 'No timer. Take your time.',
    steerHint: 'Steer hook',
    actionHint: 'Cast / Reel / Sink',
    practiceNote:
      'Local practice · Coins do not buy API credit · Progress saved per account in this browser',
  },
  merge: {
    title: 'TOKEN 2048',
    subtitle: 'Merge numbers. Make room. Find your next good move.',
    score: 'SCORE',
    moves: 'MOVES',
    boardLabel:
      '2048 board. Use arrows or WASD to move, swipe on mobile, P to pause.',
    empty: 'Empty',
    paused: 'The next move can wait.',
    pauseHint: 'Your game is paused. Come back when you are ready.',
    won: '2048. Beautifully done!',
    lost: 'A game well played.',
    result: 'You scored {score} points.',
    resume: 'Keep playing',
    continue: 'Go for a bigger number',
    restart: 'New game',
    undo: 'Undo a move',
    touchControls: 'Touch direction controls',
    up: 'Move up',
    down: 'Move down',
    left: 'Move left',
    right: 'Move right',
    guideTitle: 'Small tiles. Big possibilities.',
    guide:
      'Bring matching numbers together to merge them. Each valid move adds a new tile. Make a 2048 tile to clear the challenge.',
    tip1: 'Each tile can merge only once per move.',
    tip2: 'Keep big numbers in a corner and leave room to move.',
    tip3: 'Use arrows, WASD, swipes, or a controller D-pad.',
    keyboard: 'Arrow keys to move',
    alternative: 'WASD also supported',
    moveResult: 'Score {score}, largest tile {tile}',
    undone: 'Last move undone',
    newGame: 'A new game started',
  },
tower,
}

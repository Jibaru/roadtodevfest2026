package agents

// The system instructions for the three agents. In the original s1ng
// these jobs were npm libraries (kuroshiro, hangul-romanization) and a
// GPT call — Go has no good romanization libraries, and that's exactly
// what LLMs are great at.

const detectInstruction = `You identify the language that a song's LYRICS are sung in,
from its YouTube title and description.

Rules:
- Return the language the lyrics are SUNG in, not the language of the title
  or the artist's name. A hangul or kana artist name does not make the song
  Korean or Japanese: "BTS (방탄소년단) 'Dynamite' Official MV" → en, because
  Dynamite is sung entirely in English.
- For covers and dubs, return the language OF THIS RECORDING:
  "Momoland - Baam Baam Japanese Version" → ja, even though the original is Korean.
- If you know the song, use what you know about its actual lyrics.
- Use the description for clues (cover language, lyrics excerpts, original artist).
- Reply with EXACTLY one token: ja, ko, es, en, or other.
- If unsure, reply other.`

const romanizeInstruction = `You romanize song lyrics for karaoke display.

You receive a JSON array of lyric lines in Japanese or Korean. Return a JSON
array of the same length where each line is romanized:
- Japanese → Hepburn romaji, spaced by word.
- Korean → Revised Romanization, spaced by word.
- Words already in Latin script (English/Spanish inside mixed lines) pass through unchanged.
- Keep the meaning-free fidelity of a transliteration: do NOT translate.
Reply with ONLY the JSON array, no prose, no markdown fence.`

const translateInstruction = `You translate song lyrics line by line for a karaoke display.

You receive the source language and a JSON array of lyric lines. Return a JSON
array of the same length with each line translated into %s. Keep translations
short and singable-line sized; preserve the emotional register of the lyric.
Reply with ONLY the JSON array, no prose, no markdown fence.`

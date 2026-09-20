// Pierre supports every Shiki language by default. Alias Lens compares JSON
// catalogs only, so this small adapter keeps unrelated language grammars out of
// the embedded binary.
import {
  createBundledHighlighter,
  createCssVariablesTheme,
  createSingletonShorthands,
  getTokenStyleObject,
  stringifyTokenStyle,
} from '@shikijs/core';
import { createJavaScriptRegexEngine } from '@shikijs/engine-javascript';
import { createOnigurumaEngine } from '@shikijs/engine-oniguruma';
import json from '@shikijs/langs/json';

export const bundledLanguages = {
  json: async () => ({ default: json }),
};

export const createHighlighter = createBundledHighlighter({
  langs: bundledLanguages,
  themes: {},
  engine: createJavaScriptRegexEngine,
});

const singleton = createSingletonShorthands(createHighlighter);

export const codeToHtml = singleton.codeToHtml;
export {
  createCssVariablesTheme,
  createJavaScriptRegexEngine,
  createOnigurumaEngine,
  getTokenStyleObject,
  stringifyTokenStyle,
};

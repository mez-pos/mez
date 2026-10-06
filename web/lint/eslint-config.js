// The frontends' lint (make typecheck runs it): the rules of hooks over the
// three apps and shared, parsed by typescript-eslint.
//
// Why a workspace of its own: the apps typecheck with TypeScript 7, which has
// no JavaScript compiler API, and typescript-eslint parses through that API.
// This package holds the TypeScript 5 it needs, nested here so the apps'
// own `tsc` stays 7. It is only a parser: no rule here reads types.

import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default [
  { ignores: ["**/dist/**", "**/node_modules/**", "shared/api-types.ts"] },
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: { ecmaFeatures: { jsx: true }, sourceType: "module" },
    },
    plugins: { "react-hooks": reactHooks },
    rules: {
      // The React Compiler's rules, as warnings: this build has no compiler,
      // and what they flag (state reset in an effect, a ref read in render)
      // is deliberate here, not a bug.
      ...Object.fromEntries(Object.keys(reactHooks.configs.flat["recommended-latest"].rules).map((rule) => [rule, "warn"])),
      // The rules of hooks themselves gate the build.
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "error",
    },
  },
];

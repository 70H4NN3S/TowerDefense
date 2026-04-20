import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'
import prettier from 'eslint-config-prettier'

export default defineConfig([
  globalIgnores(['dist', 'coverage']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.strict,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
      prettier,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      // No implicit any — violations are errors, not warnings
      '@typescript-eslint/no-explicit-any': 'error',
      // Consistent type-only imports
      '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports' }],
      // Prefer const over let when variable is never reassigned
      'prefer-const': 'error',
      'no-var': 'error',
      // Warn on console.log left in production code
      'no-console': 'warn',
    },
  },
])

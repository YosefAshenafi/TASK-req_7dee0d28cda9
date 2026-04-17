import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    environmentOptions: {
      jsdom: {
        // Allow inline <script> tags to execute so app.js function declarations
        // land on window and are callable from tests.
        runScripts: 'dangerously',
      },
    },
    globals: true,
    include: ['**/*.test.js'],
  },
});

/**
 * Frontend unit tests for static/js/app.js
 *
 * Why new Function instead of script injection:
 *   jsdom's runScripts:'dangerously' executes scripts in an isolated VM context.
 *   Functions defined there land on a sandbox window, not on the test module's
 *   `window` or `document.defaultView`.  Using new Function('window','document',
 *   code) evaluates the source in the test's own global scope so that free
 *   variables (`confirm`, `location`, `setTimeout`) resolve to the same globals
 *   that vi.stubGlobal / vi.useFakeTimers patch, and DOM mutations are visible
 *   to test assertions.
 *
 *   loadApp() is kept only for the autorefresh tests: the top-level
 *   `if (document.body.dataset.autorefresh) setTimeout(...)` must run *after*
 *   the attribute is set, so re-evaluating the source inside jsdom for those
 *   cases is correct.  All other tests use the module-level factory.
 */

import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';
import { afterEach, describe, it, expect, vi } from 'vitest';

const __dirname = dirname(fileURLToPath(import.meta.url));
const appSrc = readFileSync(join(__dirname, 'app.js'), 'utf-8');

// ---------------------------------------------------------------------------
// Evaluate app.js once in the test's global context.
// This registers module-level event listeners (backdrop click, Escape key,
// DOMContentLoaded) on the shared jsdom document and returns the four public
// functions for direct use in tests.
// ---------------------------------------------------------------------------
const { confirmAction, openModal, closeModal, validateTracking } =
  new Function('window', 'document',
    appSrc + '\nreturn { confirmAction, openModal, closeModal, validateTracking };',
  )(window, document);

/**
 * Re-run app.js as an injected <script> so that synchronous top-level code
 * (specifically the autorefresh setTimeout guard) executes against the current
 * DOM state.  Only used by the dashboard auto-refresh tests.
 */
function loadApp() {
  const script = document.createElement('script');
  script.textContent = appSrc;
  document.head.appendChild(script);
}

function resetDocument(html = '') {
  document.body.innerHTML = html;
  document.body.removeAttribute('data-autorefresh');
  document.querySelectorAll('script').forEach((s) => s.remove());
}

// ── confirmAction ─────────────────────────────────────────────────────────────

describe('confirmAction', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('submits the form when the user confirms', () => {
    resetDocument('<form id="f"></form>');
    const confirmSpy = vi.fn(() => true);
    vi.stubGlobal('confirm', confirmSpy);
    const form = document.getElementById('f');
    const submitSpy = vi.spyOn(form, 'submit').mockImplementation(() => {});

    confirmAction('Delete this item?', form);

    expect(confirmSpy).toHaveBeenCalledWith('Delete this item?');
    expect(submitSpy).toHaveBeenCalledOnce();
  });

  it('does not submit when the user cancels', () => {
    resetDocument('<form id="f"></form>');
    vi.stubGlobal('confirm', vi.fn(() => false));
    const form = document.getElementById('f');
    const submitSpy = vi.spyOn(form, 'submit').mockImplementation(() => {});

    confirmAction('Delete?', form);

    expect(submitSpy).not.toHaveBeenCalled();
  });

  it('falls back to the default confirmation message when none provided', () => {
    resetDocument('<form id="f"></form>');
    const confirmSpy = vi.fn(() => false);
    vi.stubGlobal('confirm', confirmSpy);
    const form = document.getElementById('f');
    vi.spyOn(form, 'submit').mockImplementation(() => {});

    confirmAction(null, form);

    expect(confirmSpy).toHaveBeenCalledWith('Are you sure?');
  });
});

// ── openModal / closeModal ────────────────────────────────────────────────────

describe('openModal / closeModal', () => {
  it('adds the "open" class when openModal is called', () => {
    resetDocument('<div id="myModal" class="modal-backdrop"></div>');
    openModal('myModal');
    expect(document.getElementById('myModal').classList.contains('open')).toBe(true);
  });

  it('removes the "open" class when closeModal is called', () => {
    resetDocument('<div id="myModal" class="modal-backdrop open"></div>');
    closeModal('myModal');
    expect(document.getElementById('myModal').classList.contains('open')).toBe(false);
  });

  it('is a no-op when the element does not exist', () => {
    resetDocument('');
    expect(() => openModal('nonExistent')).not.toThrow();
    expect(() => closeModal('nonExistent')).not.toThrow();
  });
});

// ── modal backdrop / keyboard ─────────────────────────────────────────────────
// These tests exercise event listeners registered by the module-level factory.

describe('backdrop click closes modal', () => {
  it('removes "open" when the backdrop itself is clicked', () => {
    resetDocument('<div id="bd" class="modal-backdrop open"></div>');

    document.getElementById('bd').dispatchEvent(
      new MouseEvent('click', { bubbles: true }),
    );

    expect(document.getElementById('bd').classList.contains('open')).toBe(false);
  });

  it('does not close when a child element inside the backdrop is clicked', () => {
    resetDocument(
      '<div id="bd" class="modal-backdrop open"><button id="btn">ok</button></div>',
    );

    document.getElementById('btn').dispatchEvent(
      new MouseEvent('click', { bubbles: true }),
    );

    expect(document.getElementById('bd').classList.contains('open')).toBe(true);
  });
});

describe('Escape key closes all open modals', () => {
  it('removes "open" from every open modal on Escape', () => {
    resetDocument(
      '<div class="modal-backdrop open" id="m1"></div>' +
        '<div class="modal-backdrop open" id="m2"></div>',
    );

    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }),
    );

    expect(document.getElementById('m1').classList.contains('open')).toBe(false);
    expect(document.getElementById('m2').classList.contains('open')).toBe(false);
  });
});

// ── validateTracking ──────────────────────────────────────────────────────────

describe('validateTracking', () => {
  function makeInput(value) {
    const wrap = document.createElement('div');
    const input = document.createElement('input');
    input.value = value;
    const err = document.createElement('span');
    err.className = 'form-error';
    wrap.appendChild(input);
    wrap.appendChild(err);
    document.body.appendChild(wrap);
    return input;
  }

  it('flags tracking numbers shorter than 8 characters', () => {
    resetDocument('');
    const input = makeInput('ABC12');
    validateTracking(input);
    expect(input.classList.contains('is-error')).toBe(true);
    expect(input.parentElement.querySelector('.form-error').textContent).toMatch(/8.30/);
  });

  it('flags tracking numbers longer than 30 characters', () => {
    resetDocument('');
    const input = makeInput('A'.repeat(31));
    validateTracking(input);
    expect(input.classList.contains('is-error')).toBe(true);
  });

  it('flags non-alphanumeric characters', () => {
    resetDocument('');
    const input = makeInput('TRACK-12345');
    validateTracking(input);
    expect(input.classList.contains('is-error')).toBe(true);
  });

  it('clears the error for a valid alphanumeric tracking number', () => {
    resetDocument('');
    const input = makeInput('VALIDTRACK123');
    input.classList.add('is-error');
    input.parentElement.querySelector('.form-error').textContent = 'old error';
    validateTracking(input);
    expect(input.classList.contains('is-error')).toBe(false);
    expect(input.parentElement.querySelector('.form-error').textContent).toBe('');
  });

  it('accepts an empty value without showing an error', () => {
    resetDocument('');
    const input = makeInput('');
    validateTracking(input);
    expect(input.classList.contains('is-error')).toBe(false);
  });
});

// ── tracking blur listener (wired up on DOMContentLoaded) ────────────────────

describe('tracking blur listener', () => {
  it('validates on blur when input has data-validate="tracking"', () => {
    resetDocument(
      '<div>' +
        '<input id="t" data-validate="tracking" value="BAD!" />' +
        '<span class="form-error"></span>' +
      '</div>',
    );
    document.dispatchEvent(new Event('DOMContentLoaded'));

    document.getElementById('t').dispatchEvent(new Event('blur'));

    expect(document.getElementById('t').classList.contains('is-error')).toBe(true);
  });
});

// ── flash auto-dismiss ────────────────────────────────────────────────────────

describe('flash auto-dismiss', () => {
  afterEach(() => vi.useRealTimers());

  it('schedules a 4-second fade for flash-success', () => {
    vi.useFakeTimers();
    resetDocument('<div class="flash flash-success">Saved!</div>');
    document.dispatchEvent(new Event('DOMContentLoaded'));

    expect(vi.getTimerCount()).toBeGreaterThanOrEqual(1);
  });

  it('does not schedule a fade for flash-error messages', () => {
    vi.useFakeTimers();
    resetDocument('<div class="flash flash-error">Error!</div>');
    document.dispatchEvent(new Event('DOMContentLoaded'));

    expect(vi.getTimerCount()).toBe(0);
  });

  it('removes the flash element after the fade completes', () => {
    vi.useFakeTimers();
    resetDocument('<div class="flash flash-success">Saved!</div>');
    document.dispatchEvent(new Event('DOMContentLoaded'));

    vi.advanceTimersByTime(5000);

    expect(document.querySelector('.flash')).toBeNull();
  });
});

// ── required-field submit guard ───────────────────────────────────────────────

describe('required-field submit guard', () => {
  function setupForm() {
    resetDocument(
      '<form id="gf" data-require="reason">' +
        '<textarea name="reason"></textarea>' +
        '<span class="form-error"></span>' +
      '</form>',
    );
    document.dispatchEvent(new Event('DOMContentLoaded'));
    return document.getElementById('gf');
  }

  it('prevents submission and shows an error when required field is blank', () => {
    const form = setupForm();
    let prevented = false;
    form.addEventListener('submit', (e) => { prevented = e.defaultPrevented; });

    form.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));

    expect(prevented).toBe(true);
    expect(form.querySelector('[name="reason"]').classList.contains('is-error')).toBe(true);
  });

  it('allows submission when the required field has content', () => {
    const form = setupForm();
    form.querySelector('[name="reason"]').value = 'customer requested cancellation';
    let prevented = false;
    form.addEventListener('submit', (e) => { prevented = e.defaultPrevented; });

    form.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }));

    expect(prevented).toBe(false);
  });
});

// ── dashboard auto-refresh cadence ───────────────────────────────────────────
// loadApp() re-runs the synchronous top-level check after the attribute is set.

describe('dashboard auto-refresh', () => {
  afterEach(() => vi.useRealTimers());

  it('registers a 60-second reload when data-autorefresh is set on body', () => {
    vi.useFakeTimers();
    resetDocument('');
    document.body.setAttribute('data-autorefresh', 'true');
    loadApp();

    expect(vi.getTimerCount()).toBeGreaterThanOrEqual(1);
  });

  it('does not register the reload when data-autorefresh is absent', () => {
    vi.useFakeTimers();
    resetDocument('');
    loadApp();

    expect(vi.getTimerCount()).toBe(0);
  });
});

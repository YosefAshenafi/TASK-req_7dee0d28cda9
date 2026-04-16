// FulfillOps client-side JS

// ── Confirm dialogs ──────────────────────────────────────────
function confirmAction(message, form) {
  if (confirm(message || 'Are you sure?')) {
    form.submit();
  }
}

// ── Modal helpers ─────────────────────────────────────────────
function openModal(id) {
  const el = document.getElementById(id);
  if (el) el.classList.add('open');
}

function closeModal(id) {
  const el = document.getElementById(id);
  if (el) el.classList.remove('open');
}

// Close modal on backdrop click
document.addEventListener('click', function(e) {
  if (e.target.classList.contains('modal-backdrop')) {
    e.target.classList.remove('open');
  }
});

// Close modal on Escape
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') {
    document.querySelectorAll('.modal-backdrop.open').forEach(function(m) {
      m.classList.remove('open');
    });
  }
});

// ── Tracking number validation ────────────────────────────────
function validateTracking(input) {
  const val = input.value.trim();
  const err = input.parentElement.querySelector('.form-error');
  if (val && !/^[A-Za-z0-9]{8,30}$/.test(val)) {
    if (err) err.textContent = 'Tracking number must be 8-30 alphanumeric characters.';
    input.classList.add('is-error');
  } else {
    if (err) err.textContent = '';
    input.classList.remove('is-error');
  }
}

// ── Auto-dismiss flash messages ───────────────────────────────
document.addEventListener('DOMContentLoaded', function() {
  const flash = document.querySelector('.flash');
  if (flash && flash.classList.contains('flash-success')) {
    setTimeout(function() {
      flash.style.transition = 'opacity .4s';
      flash.style.opacity = '0';
      setTimeout(function() { flash.remove(); }, 400);
    }, 4000);
  }

  // Tracking number inputs
  document.querySelectorAll('input[data-validate="tracking"]').forEach(function(el) {
    el.addEventListener('blur', function() { validateTracking(el); });
  });

  // Required textarea check before form submit
  document.querySelectorAll('form[data-require]').forEach(function(form) {
    form.addEventListener('submit', function(e) {
      const field = form.querySelector('[name="' + form.dataset.require + '"]');
      if (field && !field.value.trim()) {
        e.preventDefault();
        const err = field.parentElement.querySelector('.form-error');
        if (err) err.textContent = 'This field is required.';
        field.classList.add('is-error');
        field.focus();
      }
    });
  });
});

// ── Dashboard auto-refresh ────────────────────────────────────
if (document.body.dataset.autorefresh) {
  setTimeout(function() { location.reload(); }, 60000);
}

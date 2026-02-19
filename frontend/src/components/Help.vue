<template>
  <div class="page-root">
    <div class="page-card">

      <!-- Header -->
      <div class="card-header">
        <div class="header-rule-left"></div>
        <span class="header-title">Help</span>
        <div class="header-rule-right"></div>
      </div>

      <!-- Hero image -->
      <div class="hero-area">
        <img
          draggable="false"
          src="../assets/images/take_it.webp"
          alt="Hang tight"
          class="hero-img"
        />
        <p class="hero-caption">This section is under construction.</p>
      </div>

      <!-- Divider -->
      <div class="lc-glow-line"></div>

      <!-- Placeholder info cards -->
      <div class="info-grid">
        <div class="info-card">
          <span class="info-icon">⬡</span>
          <div class="info-body">
            <p class="info-title">Documentation</p>
            <p class="info-text">Full usage guide coming in a future update.</p>
          </div>
        </div>
        <div class="info-card">
          <span class="info-icon">⬡</span>
          <div class="info-body">
            <p class="info-title">Support</p>
            <p class="info-text">Reach out via the project Discord or GitHub issues.</p>
          </div>
        </div>
        <div class="info-card">
          <span class="info-icon">⬡</span>
          <div class="info-body">
            <p class="info-title">Changelog</p>
            <p class="info-text">Release notes will appear here once available.</p>
          </div>
        </div>
      </div>

      <!-- Footer note -->
      <p class="footer-note animate-flicker">— Check back for an update —</p>

    </div>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.page-root {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  padding: 1rem;
  overflow-y: auto;
}

.page-card {
  display: flex;
  flex-direction: column;
  gap: 1.4rem;
  width: min(520px, 100%);
  padding: 1.8rem 2rem;
  background: linear-gradient(160deg, rgba(18,6,6,0.97) 0%, rgba(10,3,3,0.97) 100%);
  border: 1px solid var(--lc-border);
  border-radius: 2px;
  box-shadow: 0 8px 40px rgba(0,0,0,0.8);
}

/* Header */
.card-header {
  display: flex;
  align-items: center;
  gap: 0.8rem;
}
.header-rule-left,
.header-rule-right {
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--lc-border));
}
.header-rule-right {
  background: linear-gradient(90deg, var(--lc-border), transparent);
}
.header-title {
  font-family: "MikoDacs", sans-serif;
  font-size: 1.5rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--lc-gold);
  text-shadow: 0 0 14px rgba(200,150,44,0.5);
  white-space: nowrap;
}

/* Hero */
.hero-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
}
.hero-img {
  width: 180px;
  height: auto;
  filter: sepia(0.3) contrast(1.1);
  opacity: 0.88;
}
.hero-caption {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.8rem;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--lc-text-muted);
}

/* Info grid */
.info-grid {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.info-card {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.65rem 0.9rem;
  border: 1px solid var(--lc-border);
  border-radius: 2px;
  background: rgba(0,0,0,0.2);
  transition: border-color 0.15s, background 0.15s;
}
.info-card:hover {
  border-color: var(--lc-border-hover);
  background: rgba(120,20,20,0.07);
}
.info-icon {
  font-size: 1rem;
  color: var(--lc-gold-dim);
  flex-shrink: 0;
  line-height: 1.4;
}
.info-body {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  text-align: left;
}
.info-title {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.82rem;
  letter-spacing: 0.08em;
  color: var(--lc-cream);
  text-transform: uppercase;
}
.info-text {
  font-family: "Noto Sans", sans-serif;
  font-size: 0.74rem;
  color: var(--lc-text-muted);
}

/* Footer */
.footer-note {
  font-family: "MikoDacs", sans-serif;
  font-size: 0.7rem;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  color: var(--lc-text-muted);
  text-align: center;
}
</style>

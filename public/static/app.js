/* ═══════════════════════════════════════════════════════
   NEXUS STRATEGIC — Application
   ═══════════════════════════════════════════════════════ */

const state = {
    userPerson:    null,
    matchResults:  [],
    selectedTarget: null,
    attendees:     [],
    metrics: { sessions: 0, prospects: 0, emails: 0 },
    // Practice sandbox
    practiceTarget:  null,
    chatHistory:     [],   // [{role, content}]
    isListening:     false,
    isSpeaking:      false,
    recognition:     null,
};

// ── Helpers ──
function initials(name) {
    if (!name) return '?';
    return name.split(' ').slice(0, 2).map(w => w[0].toUpperCase()).join('');
}

function el(id) { return document.getElementById(id); }

function chips(arr, limit = 4) {
    if (!arr || !arr.length) return '';
    return arr.slice(0, limit).map(s => `<span class="ai-chip">${s}</span>`).join('');
}


// ═══════════════════════════════════════════════════════
// AUTH / LOGIN
// ═══════════════════════════════════════════════════════
el('loginForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    const email = el('loginEmail').value.trim();
    const btn = e.target.querySelector('button[type=submit]');
    btn.disabled = true;
    btn.textContent = 'Signing in…';

    // Derive display name from email
    const namePart = email.split('@')[0].replace(/[._-]/g, ' ');
    const displayName = namePart.split(' ').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');

    el('userName').textContent = displayName;
    el('userInitials').textContent = initials(displayName);

    // Small delay for feel, then boot the app
    await new Promise(r => setTimeout(r, 600));
    el('login-screen').classList.add('hidden');
    el('app-shell').classList.remove('hidden');

    navigate('dashboard');
    loadAttendees();
});


// ═══════════════════════════════════════════════════════
// NAVIGATION
// ═══════════════════════════════════════════════════════
const VIEW_TITLES = {
    dashboard: 'Dashboard',
    scout:     'Attendee Scout',
    matching:  'Strategic Match',
    email:     'Outreach Drafter',
    practice:  'Pitch Practice',
};

function navigate(viewName) {
    document.querySelectorAll('.view').forEach(v => {
        v.classList.remove('active', 'hidden');
        v.style.display = 'none';
    });

    const target = el(`view-${viewName}`);
    if (target) {
        target.style.display = '';
        target.classList.add('active');
    }

    // Update nav
    document.querySelectorAll('.nav-item').forEach(item => {
        item.classList.toggle('active', item.dataset.view === viewName);
    });

    // Update topbar
    el('topbar-title').textContent = VIEW_TITLES[viewName] || viewName;

    // Clear email badge when visiting email view
    if (viewName === 'email') {
        el('email-badge').classList.add('hidden');
    }

    // Show practice picker or session depending on state
    if (viewName === 'practice') {
        renderPracticeView();
    }
}

// Wire nav buttons
document.querySelectorAll('.nav-item[data-view]').forEach(btn => {
    btn.addEventListener('click', () => navigate(btn.dataset.view));
});


// ═══════════════════════════════════════════════════════
// DASHBOARD
// ═══════════════════════════════════════════════════════
function updateMetrics() {
    el('metric-sessions').textContent = state.metrics.sessions;
    el('metric-prospects').textContent = state.metrics.prospects;
    el('metric-emails').textContent = state.metrics.emails;
}


// ═══════════════════════════════════════════════════════
// ATTENDEES
// ═══════════════════════════════════════════════════════
async function loadAttendees() {
    try {
        const res = await fetch('/api/attendees');
        const data = await res.json();
        state.attendees = data.attendees || [];
        el('metric-attendees').textContent = state.attendees.length;
        renderAttendeeGrid(state.attendees);
    } catch (err) {
        el('attendee-grid').innerHTML = '<div class="loading-state">Failed to load profiles.</div>';
    }
}

function filterAttendees() {
    const q = el('scout-search').value.toLowerCase().trim();
    if (!q) {
        renderAttendeeGrid(state.attendees);
        return;
    }
    const filtered = state.attendees.filter(p => {
        const text = [p.name, p.title, p.company, ...(p.skills || []), ...(p.interests || [])].join(' ').toLowerCase();
        return text.includes(q);
    });
    renderAttendeeGrid(filtered);
}

function renderAttendeeGrid(list) {
    const grid = el('attendee-grid');
    el('scout-visible-count').textContent = list.length;

    if (!list.length) {
        grid.innerHTML = '<div class="loading-state">No profiles match your search.</div>';
        return;
    }

    grid.innerHTML = list.map((person, i) => {
        const matchBadge = person._score
            ? `<span class="match-badge">${(person._score * 100).toFixed(0)}% match</span>` : '';
        const skillChips = chips(person.skills, 3);
        const interestChips = chips(person.interests, 2);

        return `
        <div class="attendee-card" data-attendee-index="${i}">
            <div class="attendee-card-top">
                <div class="attendee-avatar">${initials(person.name)}</div>
                ${matchBadge}
            </div>
            <div class="attendee-name">${person.name || 'Unknown'}</div>
            <div class="attendee-role">${[person.title, person.company].filter(Boolean).join(' · ') || 'No details'}</div>
            ${skillChips || interestChips ? `<div class="attendee-chips">${skillChips}${interestChips}</div>` : ''}
            <div class="attendee-card-actions">
                <div class="attendee-draft-btn">Draft Outreach</div>
                <div class="attendee-practice-btn" data-attendee-index="${i}">Practice Pitch 🎙</div>
            </div>
        </div>`;
    }).join('');

    // Store the rendered list reference so click handlers can look up by index
    const renderedList = list;
    grid.querySelectorAll('.attendee-card[data-attendee-index]').forEach(card => {
        card.querySelector('.attendee-draft-btn').addEventListener('click', (e) => {
            e.stopPropagation();
            const idx = parseInt(card.dataset.attendeeIndex, 10);
            draftFromScout(renderedList[idx]);
        });
        card.querySelector('.attendee-practice-btn').addEventListener('click', (e) => {
            e.stopPropagation();
            const idx = parseInt(card.dataset.attendeeIndex, 10);
            startPracticeWith(renderedList[idx]);
        });
    });
}

function draftFromScout(person) {
    state.selectedTarget = person;
    navigate('email');
    renderIntelPanel(person);
    generateEmail(person);
}


// ═══════════════════════════════════════════════════════
// STRATEGIC MATCH — Profile
// ═══════════════════════════════════════════════════════
async function submitProfile() {
    const desc = el('profileDescription').value.trim();
    if (!desc) return;

    const btn = el('profileBtn');
    btn.disabled = true;
    btn.textContent = 'Processing…';

    try {
        const res = await fetch('/api/profile', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ description: desc }),
        });
        const data = await res.json();
        if (data.error) { showMatchError(data.error); return; }

        state.userPerson = data.person;
        showProfileConfigured(data.person);
    } catch (err) {
        showMatchError('Failed to process profile. Please try again.');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Confirm Profile →';
    }
}

function showProfileConfigured(person) {
    el('profile-form').classList.add('hidden');
    el('profile-configured').classList.remove('hidden');
    el('profile-status-badge').classList.remove('hidden');

    el('profile-preview').innerHTML = `
        <div class="preview-name">${person.name || 'Your Profile'}</div>
        <div class="preview-role">${[person.title, person.company].filter(Boolean).join(' at ') || ''}</div>
        <div class="attendee-chips">${chips(person.skills, 4)}</div>
    `;
}

function resetProfile() {
    state.userPerson = null;
    el('profile-form').classList.remove('hidden');
    el('profile-configured').classList.add('hidden');
    el('profile-status-badge').classList.add('hidden');
}


// ═══════════════════════════════════════════════════════
// STRATEGIC MATCH — Find
// ═══════════════════════════════════════════════════════
async function submitMatch() {
    const desc = el('matchDescription').value.trim();
    if (!desc) return;

    if (!state.userPerson) {
        el('profileDescription').focus();
        el('profile-panel').style.outline = '2px solid var(--primary)';
        setTimeout(() => el('profile-panel').style.outline = '', 1500);
        return;
    }

    const btn = el('matchBtn');
    btn.disabled = true;
    btn.textContent = 'Searching…';

    el('results-placeholder').classList.add('hidden');
    el('match-loading').classList.remove('hidden');
    el('match-results-list').innerHTML = '';

    const messages = ['Processing semantic intelligence…', 'Generating embedding vector…', 'Scanning 52 profiles…', 'Ranking by relevance…'];
    let mi = 0;
    const msgInterval = setInterval(() => {
        el('match-loading-msg').textContent = messages[Math.min(mi++, messages.length - 1)];
    }, 900);

    try {
        const res = await fetch('/api/match', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ description: desc }),
        });
        const data = await res.json();

        clearInterval(msgInterval);
        el('match-loading').classList.add('hidden');

        if (data.error) { showMatchError(data.error); return; }

        state.matchResults = data.matches || [];
        state.metrics.sessions++;
        state.metrics.prospects += state.matchResults.length;
        updateMetrics();

        renderMatchResults(state.matchResults);
    } catch (err) {
        clearInterval(msgInterval);
        el('match-loading').classList.add('hidden');
        showMatchError('Failed to find matches. Please try again.');
    } finally {
        btn.disabled = false;
        btn.textContent = 'Surface Prospects →';
    }
}

function renderMatchResults(results) {
    const container = el('match-results-list');

    if (!results.length) {
        container.innerHTML = '<div class="loading-state">No matches found. Try refining your description.</div>';
        return;
    }

    container.innerHTML = results.map((r, i) => {
        const p = r.person;
        const pct = (r.similarity * 100).toFixed(0);
        return `
        <div class="match-result-card">
            <div class="match-result-top">
                <div>
                    <div class="match-result-name">${i + 1}. ${p.name || 'Unknown'}</div>
                    <div class="match-result-role">${[p.title, p.company].filter(Boolean).join(' · ') || ''}</div>
                </div>
                <span class="match-badge">${pct}%</span>
            </div>
            <div class="match-score-bar-wrap">
                <div class="match-score-label">Relevance Score</div>
                <div class="match-score-track">
                    <div class="match-score-fill" style="width: ${pct}%"></div>
                </div>
            </div>
            ${p.skills && p.skills.length ? `<div class="match-chips">${chips(p.skills, 4)}</div>` : ''}
            <div class="match-result-actions">
                <button class="draft-btn" data-match-index="${i}">Draft Outreach →</button>
                <button class="practice-btn" data-match-index="${i}">Practice Pitch 🎙</button>
            </div>
        </div>`;
    }).join('');

    // Attach listeners after rendering to avoid JSON-in-attribute issues
    container.querySelectorAll('.draft-btn[data-match-index]').forEach(btn => {
        btn.addEventListener('click', () => {
            const idx = parseInt(btn.dataset.matchIndex, 10);
            selectAndDraft(state.matchResults[idx].person);
        });
    });
    container.querySelectorAll('.practice-btn[data-match-index]').forEach(btn => {
        btn.addEventListener('click', () => {
            const idx = parseInt(btn.dataset.matchIndex, 10);
            startPracticeWith(state.matchResults[idx].person);
        });
    });
}

function showMatchError(msg) {
    el('match-results-list').innerHTML = `<div class="error-banner">${msg}</div>`;
    el('results-placeholder').classList.add('hidden');
}


// ═══════════════════════════════════════════════════════
// EMAIL DRAFTER
// ═══════════════════════════════════════════════════════
function selectAndDraft(person) {
    state.selectedTarget = person;
    el('email-badge').classList.remove('hidden');
    navigate('email');
    renderIntelPanel(person);
    generateEmail(person);
}

function renderIntelPanel(person) {
    const goals = (person.goals || []).map(g => `<div class="intel-goal">${g}</div>`).join('');
    el('intel-content').innerHTML = `
        <div class="intel-name">${person.name || 'Unknown'}</div>
        <div class="intel-role">${[person.title, person.company].filter(Boolean).join(' · ') || ''}</div>
        ${person.skills && person.skills.length ? `
        <div class="intel-section">
            <div class="intel-section-label">Skills</div>
            <div class="intel-chips">${chips(person.skills, 8)}</div>
        </div>` : ''}
        ${person.interests && person.interests.length ? `
        <div class="intel-section">
            <div class="intel-section-label">Interests</div>
            <div class="intel-chips">${chips(person.interests, 6)}</div>
        </div>` : ''}
        ${goals ? `
        <div class="intel-section">
            <div class="intel-section-label">Goals</div>
            ${goals}
        </div>` : ''}
    `;
}

async function generateEmail(toPerson) {
    const from = state.userPerson;

    el('emailText').value = '';
    el('email-generating').classList.remove('hidden');
    el('email-actions').classList.add('hidden');

    // If no user profile, use a generic sender
    const sender = from || { name: el('userName').textContent, description: 'conference attendee' };

    try {
        const res = await fetch('/api/email', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ from: sender, to: toPerson }),
        });
        const data = await res.json();

        el('email-generating').classList.add('hidden');

        if (data.error) {
            el('emailText').value = `Error: ${data.error}`;
            return;
        }

        el('emailText').value = data.email;
        el('email-actions').classList.remove('hidden');

        state.metrics.emails++;
        updateMetrics();
    } catch (err) {
        el('email-generating').classList.add('hidden');
        el('emailText').value = 'Failed to generate email. Please try again.';
    }
}

function regenerateEmail() {
    if (state.selectedTarget) generateEmail(state.selectedTarget);
}

function copyEmail(e) {
    const text = el('emailText').value;
    navigator.clipboard.writeText(text).then(() => {
        const btn = e ? e.target : document.querySelector('[onclick="copyEmail()"]');
        const orig = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(() => btn.textContent = orig, 2000);
    });
}


// ═══════════════════════════════════════════════════════
// PROFILE MODAL
// ═══════════════════════════════════════════════════════
function showProfileModal() {
    const modal = el('profile-modal');
    const content = el('modal-profile-content');

    if (state.userPerson) {
        const p = state.userPerson;
        content.innerHTML = `
            <div style="margin-bottom: 1rem;">
                <div class="preview-name" style="font-size: 1.1rem;">${p.name || 'Your Profile'}</div>
                <div class="preview-role" style="margin-bottom: 0.875rem;">${[p.title, p.company].filter(Boolean).join(' at ') || ''}</div>
                ${p.skills && p.skills.length ? `
                <div class="intel-section">
                    <div class="intel-section-label" style="margin-bottom: 0.4rem;">Skills</div>
                    <div class="intel-chips">${chips(p.skills, 8)}</div>
                </div>` : ''}
                ${p.interests && p.interests.length ? `
                <div class="intel-section" style="margin-top: 0.75rem;">
                    <div class="intel-section-label" style="margin-bottom: 0.4rem;">Interests</div>
                    <div class="intel-chips">${chips(p.interests, 6)}</div>
                </div>` : ''}
            </div>
            <button class="btn-ghost" onclick="hideProfileModal(); navigate('matching'); resetProfile();">Edit Profile</button>
        `;
    } else {
        content.innerHTML = `
            <p style="color: var(--on-surface-variant); font-size: 0.875rem; margin-bottom: 1rem;">No profile configured yet. Head to Strategic Match to set up your profile.</p>
            <button class="btn-primary" style="margin-top: 0;" onclick="hideProfileModal(); navigate('matching');">Set Up Profile →</button>
        `;
    }

    modal.classList.remove('hidden');
}

function hideProfileModal() {
    el('profile-modal').classList.add('hidden');
}

function closeModalOnBackdrop(e) {
    if (e.target === el('profile-modal')) hideProfileModal();
}


// ═══════════════════════════════════════════════════════
// PITCH PRACTICE
// ═══════════════════════════════════════════════════════

// Voice pool — assigned deterministically by name so it's stable across sessions
const PRACTICE_VOICES = ['Kore', 'Zephyr', 'Puck', 'Charon', 'Fenrir', 'Leda', 'Sulafat'];

function voiceForPerson(name) {
    let h = 0;
    for (const c of (name || '')) h = (Math.imul(h, 31) + c.charCodeAt(0)) | 0;
    return PRACTICE_VOICES[Math.abs(h) % PRACTICE_VOICES.length];
}

function startPracticeWith(person) {
    state.practiceTarget = person;
    state.chatHistory = [];
    navigate('practice');
}

function resetPractice() {
    state.practiceTarget = null;
    state.chatHistory = [];
    stopMic();
    renderPracticeView();
}

function restartConversation() {
    state.chatHistory = [];
    stopMic();
    el('practice-transcript').innerHTML = '';
    openingGreeting();
}

function renderPracticeView() {
    if (!state.practiceTarget) {
        el('practice-picker').classList.remove('hidden');
        el('practice-session').classList.add('hidden');
        return;
    }

    el('practice-picker').classList.add('hidden');
    el('practice-session').classList.remove('hidden');

    const p = state.practiceTarget;
    el('practice-persona-card').innerHTML = `
        <div style="display:flex;align-items:center;gap:0.75rem;margin-bottom:0.75rem;">
            <div class="attendee-avatar" style="width:40px;height:40px;font-size:0.875rem;">${initials(p.name)}</div>
            <div>
                <div style="font-weight:600;font-size:0.9rem;">${p.name}</div>
                <div style="font-size:0.78rem;color:var(--on-surface-variant);">${[p.title, p.company].filter(Boolean).join(' · ')}</div>
            </div>
        </div>
        ${p.skills && p.skills.length ? `<div class="intel-chips" style="margin-bottom:0.5rem;">${chips(p.skills, 4)}</div>` : ''}
        ${p.interests && p.interests.length ? `<div class="intel-chips">${chips(p.interests, 3)}</div>` : ''}
    `;

    // Start with a greeting if transcript is empty
    if (el('practice-transcript').innerHTML.trim() === '') {
        openingGreeting();
    }
}

function openingGreeting() {
    const p = state.practiceTarget;
    const greeting = `Hi! I'm ${p.name}, ${p.title} at ${p.company}. Great to meet you here at the conference — what brings you to Global Tech Summit this year?`;
    appendMessage('model', greeting);
    state.chatHistory.push({ role: 'model', content: greeting });
    speakText(greeting);
}

// ── Transcript rendering ──

function appendMessage(role, content) {
    const transcript = el('practice-transcript');
    const div = document.createElement('div');
    div.className = `practice-message practice-message--${role}`;

    const label = role === 'user' ? 'You' : (state.practiceTarget ? state.practiceTarget.name.split(' ')[0] : 'Them');
    div.innerHTML = `
        <div class="practice-msg-label">${label}</div>
        <div class="practice-msg-bubble">${content}</div>
    `;
    transcript.appendChild(div);
    transcript.scrollTop = transcript.scrollHeight;
}

function setStatus(text, active = false) {
    const el2 = el('practice-status-text');
    el2.textContent = text;
    el('practice-status').classList.toggle('practice-status--active', active);
}

// ── Sending messages ──

async function sendUserMessage(text) {
    if (!text.trim() || !state.practiceTarget) return;

    appendMessage('user', text);
    state.chatHistory.push({ role: 'user', content: text });

    setStatus(`${state.practiceTarget.name.split(' ')[0]} is thinking…`, true);
    el('mic-btn').disabled = true;

    try {
        const res = await fetch('/api/chat', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                person: state.practiceTarget,
                history: state.chatHistory.slice(0, -1), // exclude the just-added user msg
                message: text,
            }),
        });
        const data = await res.json();
        if (data.error) {
            setStatus(`Error: ${data.error}`);
            return;
        }

        const reply = data.reply;
        appendMessage('model', reply);
        state.chatHistory.push({ role: 'model', content: reply });
        setStatus('');
        await speakText(reply);
    } catch (err) {
        setStatus('Failed to get response. Please try again.');
    } finally {
        el('mic-btn').disabled = false;
    }
}

function sendTextMessage() {
    const input = el('practice-text-input');
    const text = input.value.trim();
    if (!text) return;
    input.value = '';
    sendUserMessage(text);
}

// ── TTS playback ──

async function speakText(text) {
    if (!text) return;
    state.isSpeaking = true;
    setStatus('Speaking…', true);

    try {
        const voice = voiceForPerson(state.practiceTarget?.name);
        const res = await fetch('/api/tts', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ text, voice }),
        });

        if (!res.ok) {
            // TTS failed silently — the text is still visible in the transcript
            console.warn('TTS request failed:', res.status);
            return;
        }

        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const audio = new Audio(url);

        await new Promise((resolve) => {
            audio.onended = resolve;
            audio.onerror = resolve;
            audio.play().catch(resolve);
        });

        URL.revokeObjectURL(url);
    } catch (err) {
        console.warn('TTS playback error:', err);
    } finally {
        state.isSpeaking = false;
        setStatus('');
    }
}

// ── Speech recognition (STT) ──

function initRecognition() {
    const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!SR) return null;

    const rec = new SR();
    rec.continuous = false;
    rec.interimResults = true;
    rec.lang = 'en-US';

    rec.onstart = () => {
        state.isListening = true;
        el('mic-btn').classList.add('mic-button--recording');
        el('mic-label').textContent = 'Listening…';
        setStatus('Listening — speak now', true);
    };

    rec.onresult = (event) => {
        let interim = '';
        let final = '';
        for (const result of event.results) {
            if (result.isFinal) final += result[0].transcript;
            else interim += result[0].transcript;
        }
        setStatus(final || interim || 'Listening…', true);
        if (final) {
            stopMic();
            sendUserMessage(final.trim());
        }
    };

    rec.onerror = (event) => {
        console.warn('Speech recognition error:', event.error);
        if (event.error === 'not-allowed') {
            setStatus('Microphone access denied. Use the text input below.');
        } else {
            setStatus('');
        }
        stopMic();
    };

    rec.onend = () => {
        stopMic();
    };

    return rec;
}

function toggleMic() {
    if (state.isSpeaking) return; // don't interrupt TTS

    if (state.isListening) {
        stopMic();
        return;
    }

    const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
    if (!SR) {
        setStatus('Speech recognition not supported in this browser. Use the text input below.');
        return;
    }

    if (!state.recognition) {
        state.recognition = initRecognition();
    }

    try {
        state.recognition.start();
    } catch (e) {
        // Already started — stop and restart
        state.recognition.stop();
        setTimeout(() => state.recognition && state.recognition.start(), 200);
    }
}

function stopMic() {
    state.isListening = false;
    el('mic-btn').classList.remove('mic-button--recording');
    el('mic-label').textContent = 'Tap to Speak';
    if (state.recognition) {
        try { state.recognition.stop(); } catch (_) {}
    }
}

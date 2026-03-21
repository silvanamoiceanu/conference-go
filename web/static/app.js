// Conference Connection Matcher Frontend

document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('connectionForm');
    const submitBtn = document.getElementById('submitBtn');
    const statusSection = document.getElementById('statusSection');
    const progressBar = document.getElementById('progressBar');
    const statusMessage = document.getElementById('statusMessage');
    const resultsSection = document.getElementById('resultsSection');
    const resultsContainer = document.getElementById('resultsContainer');

    form.addEventListener('submit', async function(e) {
        e.preventDefault();

        const description = document.getElementById('description').value.trim();
        if (!description) {
            showError('Please enter a description for your ideal connection.');
            return;
        }

        // Show loading state
        showLoading();

        try {
            const response = await fetch('/api/match', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ description: description })
            });

            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }

            const data = await response.json();
            showResults(data);

        } catch (error) {
            console.error('Error:', error);
            showError('An error occurred while processing your request. Please try again.');
        }
    });

    function showLoading() {
        submitBtn.disabled = true;
        submitBtn.textContent = 'Processing...';
        statusSection.style.display = 'block';
        resultsSection.style.display = 'none';
        progressBar.style.width = '0%';
        statusMessage.textContent = 'Initializing...';

        // Simulate progress updates
        let progress = 0;
        const progressInterval = setInterval(() => {
            progress += Math.random() * 15;
            if (progress > 90) progress = 90;
            progressBar.style.width = progress + '%';

            const messages = [
                'Initializing...',
                'Loading AI models...',
                'Processing your description...',
                'Generating embeddings...',
                'Searching for matches...',
                'Finalizing results...'
            ];
            const messageIndex = Math.floor(progress / 15);
            if (messageIndex < messages.length) {
                statusMessage.textContent = messages[messageIndex];
            }
        }, 500);

        // Store interval for cleanup
        window.progressInterval = progressInterval;
    }

    function showResults(data) {
        // Clear progress interval
        if (window.progressInterval) {
            clearInterval(window.progressInterval);
        }

        submitBtn.disabled = false;
        submitBtn.textContent = 'Find Matches';
        statusSection.style.display = 'none';
        resultsSection.style.display = 'block';

        if (data.error) {
            showError(data.error);
            return;
        }

        progressBar.style.width = '100%';
        statusMessage.textContent = 'Complete!';

        // Clear previous results
        resultsContainer.innerHTML = '';

        if (!data.matches || data.matches.length === 0) {
            resultsContainer.innerHTML = '<p>No matches found. Try adjusting your description.</p>';
            return;
        }

        // Display matches
        data.matches.forEach((match, index) => {
            const matchCard = createMatchCard(match, index + 1);
            resultsContainer.appendChild(matchCard);
        });
    }

    function createMatchCard(match, rank) {
        const card = document.createElement('div');
        card.className = 'match-card';

        const similarityPercent = (match.similarity * 100).toFixed(1);

        card.innerHTML = `
            <div class="match-header">
                <div class="match-name">${rank}. ${match.person.name || 'Unknown'}</div>
                <div class="similarity-score">${similarityPercent}% match</div>
            </div>
            <div class="match-details">
                <strong>${match.person.title || 'No title'} at ${match.person.company || 'No company'}</strong>
            </div>
            ${match.person.interests && match.person.interests.length > 0 ? `
                <div class="match-interests">
                    <h4>Interests:</h4>
                    <div class="tags">
                        ${match.person.interests.map(interest => `<span class="tag">${interest}</span>`).join('')}
                    </div>
                </div>
            ` : ''}
            ${match.person.skills && match.person.skills.length > 0 ? `
                <div class="match-skills">
                    <h4>Skills:</h4>
                    <div class="tags">
                        ${match.person.skills.map(skill => `<span class="tag">${skill}</span>`).join('')}
                    </div>
                </div>
            ` : ''}
        `;

        return card;
    }

    function showError(message) {
        // Clear progress interval
        if (window.progressInterval) {
            clearInterval(window.progressInterval);
        }

        submitBtn.disabled = false;
        submitBtn.textContent = 'Find Matches';
        statusSection.style.display = 'none';
        resultsSection.style.display = 'block';

        resultsContainer.innerHTML = `<div class="error-message">${message}</div>`;
    }
});
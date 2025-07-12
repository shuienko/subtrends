// SubTrends Web Application JavaScript

document.addEventListener('DOMContentLoaded', function() {
    // Initialize form handlers
    initializeAnalyzeForm();
    initializeModelForm();
});

// Initialize the analyze form
function initializeAnalyzeForm() {
    const form = document.getElementById('analyzeForm');
    if (form) {
        form.addEventListener('submit', function(e) {
            e.preventDefault();
            const subreddit = document.getElementById('subreddit').value.trim();
            if (subreddit) {
                analyzeSubreddit(subreddit);
            }
        });
    }
}

// Initialize the model form
function initializeModelForm() {
    const form = document.getElementById('modelForm');
    if (form) {
        form.addEventListener('submit', function(e) {
            e.preventDefault();
            const model = document.getElementById('model').value;
            changeModel(model);
        });
    }
}

// Analyze a subreddit
function analyzeSubreddit(subreddit) {
    // Clean subreddit name
    subreddit = subreddit.replace(/^r\//, '');
    
    // Update form value
    const input = document.getElementById('subreddit');
    if (input) {
        input.value = subreddit;
    }
    
    // Show loading state
    showLoading();
    hideError();
    hideResults();
    
    // Prepare form data
    const formData = new FormData();
    formData.append('subreddit', subreddit);
    
    // Make API request
    fetch('/analyze', {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
    .then(data => {
        hideLoading();
        if (data.error) {
            showError(data.error);
        } else {
            showResults(data);
        }
    })
    .catch(error => {
        hideLoading();
        showError('Failed to analyze subreddit. Please try again.');
        console.error('Error:', error);
    });
}

// Change AI model
function changeModel(model) {
    const formData = new FormData();
    formData.append('model', model);
    
    fetch('/model', {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert('Error: ' + data.error, 'danger');
        } else {
            showAlert(data.message, 'success');
            // Reload page to reflect changes
            setTimeout(() => {
                window.location.reload();
            }, 1500);
        }
    })
    .catch(error => {
        showAlert('Failed to change model. Please try again.', 'danger');
        console.error('Error:', error);
    });
}

// Clear history
function clearHistory() {
    if (!confirm('Are you sure you want to clear your analysis history?')) {
        return;
    }
    
    fetch('/clear-history', {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert('Error: ' + data.error, 'danger');
        } else {
            showAlert(data.message, 'success');
            // Reload page to reflect changes
            setTimeout(() => {
                window.location.reload();
            }, 1500);
        }
    })
    .catch(error => {
        showAlert('Failed to clear history. Please try again.', 'danger');
        console.error('Error:', error);
    });
}

// Show loading state
function showLoading() {
    const loading = document.getElementById('loading');
    if (loading) {
        loading.classList.remove('d-none');
    }
    
    const button = document.getElementById('analyzeBtn');
    if (button) {
        button.disabled = true;
        button.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Analyzing...';
    }
}

// Hide loading state
function hideLoading() {
    const loading = document.getElementById('loading');
    if (loading) {
        loading.classList.add('d-none');
    }
    
    const button = document.getElementById('analyzeBtn');
    if (button) {
        button.disabled = false;
        button.innerHTML = '<i class="fas fa-magic"></i> Analyze';
    }
}

// Show results
function showResults(data) {
    const results = document.getElementById('results');
    const summary = document.getElementById('summary');
    const posts = document.getElementById('posts');
    
    if (results && summary && posts) {
        // Display summary
        summary.innerHTML = formatSummary(data.summary);
        
        // Display posts
        if (data.posts && data.posts.length > 0) {
            posts.innerHTML = formatPosts(data.posts);
        } else {
            posts.innerHTML = '';
        }
        
        results.classList.remove('d-none');
        
        // Scroll to results
        results.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
}

// Hide results
function hideResults() {
    const results = document.getElementById('results');
    if (results) {
        results.classList.add('d-none');
    }
}

// Show error
function showError(message) {
    const error = document.getElementById('error');
    const errorMessage = document.getElementById('errorMessage');
    
    if (error && errorMessage) {
        errorMessage.textContent = message;
        error.classList.remove('d-none');
        
        // Scroll to error
        error.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
}

// Hide error
function hideError() {
    const error = document.getElementById('error');
    if (error) {
        error.classList.add('d-none');
    }
}

// Format summary for display
function formatSummary(summary) {
    if (!summary) return '';
    
    // Convert markdown-like formatting to HTML
    let formatted = summary
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.*?)\*/g, '<em>$1</em>')
        .replace(/\n/g, '<br>')
        .replace(/📊 TRENDING TOPICS:/g, '<h4 class="text-primary">📊 TRENDING TOPICS:</h4>')
        .replace(/💬 COMMUNITY PULSE:/g, '<h4 class="text-info">💬 COMMUNITY PULSE:</h4>')
        .replace(/🔥 HOT TAKES:/g, '<h4 class="text-warning">🔥 HOT TAKES:</h4>');
    
    return formatted;
}

// Format posts for display
function formatPosts(posts) {
    if (!posts || posts.length === 0) {
        return '';
    }
    
    let html = '<h5><i class="fas fa-link"></i> Top Posts</h5>';
    
    const emojiNumbers = ['1️⃣', '2️⃣', '3️⃣', '4️⃣', '5️⃣', '6️⃣', '7️⃣', '8️⃣', '9️⃣', '🔟'];
    
    posts.forEach((post, index) => {
        if (index >= 10) return; // Limit to 10 posts
        
        // Check if post has required properties (using lowercase names)
        if (!post) {
            return;
        }
        
        if (!post.title) {
            return;
        }
        
        if (!post.permalink) {
            return;
        }
        
        const emoji = emojiNumbers[index] || `${index + 1}.`;
        const webLink = `https://www.reddit.com${post.permalink}`;
        
        html += `
            <a href="${webLink}" target="_blank" class="post-link">
                <div class="post-title">${emoji} ${post.title}</div>
                <div class="post-url">${webLink}</div>
            </a>
        `;
    });
    
    return html;
}

// Show alert message
function showAlert(message, type) {
    // Create alert element
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
    alertDiv.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    // Insert at top of main container
    const main = document.querySelector('main');
    if (main) {
        main.insertBefore(alertDiv, main.firstChild);
        
        // Auto-dismiss after 5 seconds
        setTimeout(() => {
            if (alertDiv.parentNode) {
                alertDiv.remove();
            }
        }, 5000);
    }
}

// Utility function to debounce API calls
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Add smooth scrolling to all internal links
document.addEventListener('DOMContentLoaded', function() {
    const links = document.querySelectorAll('a[href^="#"]');
    links.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                target.scrollIntoView({ behavior: 'smooth' });
            }
        });
    });
}); 
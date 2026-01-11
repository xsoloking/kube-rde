# Videos

This directory contains demo and tutorial videos for KubeRDE.

## Required Videos

### Main Demo Video
- [ ] `demo.mp4` - **Main Product Demo** (3-5 minutes)
  - Introduction to KubeRDE (30s)
  - Problem statement and solution (45s)
  - Key features demonstration (2 min)
  - Real-world use case (1 min)
  - Call to action (30s)

### Tutorial Series
- [ ] `tutorial-01-quick-start.mp4` - **Getting Started** (5 minutes)
  - Local installation with kind/minikube
  - First login and authentication
  - Creating your first workspace
  - Accessing services

- [ ] `tutorial-02-workspace-management.mp4` - **Workspace Management** (7 minutes)
  - Creating workspaces with different templates
  - Adding services (SSH, Jupyter, VS Code, File Browser)
  - Configuring resources (CPU, memory, storage)
  - Managing workspace lifecycle

- [ ] `tutorial-03-ssh-access.mp4` - **SSH Access** (5 minutes)
  - Installing and configuring CLI
  - SSH key setup
  - Connecting to workspaces via SSH
  - Port forwarding and SOCKS proxy

- [ ] `tutorial-04-production-deployment.mp4` - **Production Deployment** (10 minutes)
  - Deploying to GKE/EKS/AKS
  - Configuring domain and TLS certificates
  - Setting up Keycloak authentication
  - User and quota management

- [ ] `tutorial-05-operator-customization.mp4` - **Operator & CRDs** (8 minutes)
  - Understanding RDEAgent CRD
  - Creating custom agent templates
  - Declarative workspace management
  - Auto-scaling and TTL configuration

## File Specifications

### Video Format
- **Container:** MP4
- **Video codec:** H.264 (High Profile)
- **Resolution:** 1920x1080 (1080p)
- **Frame rate:** 30 fps
- **Bitrate:** 5000-8000 kbps (CBR or VBR)

### Audio Format
- **Codec:** AAC
- **Sample rate:** 48 kHz
- **Bitrate:** 128-192 kbps
- **Channels:** Stereo (2.0)

### File Size
- **Target:** < 50MB for 5-minute video
- **Maximum:** < 100MB per video

## Video Script Template

```markdown
# Video Title

**Duration:** X minutes
**Target audience:** [Beginners/Advanced users/Operators]

## Introduction (10-15s)
- Hook
- What viewers will learn

## Main Content
### Section 1 (Xm Xs)
- Point 1
- Point 2
- Demonstration

### Section 2 (Xm Xs)
- Point 1
- Point 2
- Demonstration

## Conclusion (10-15s)
- Summary
- Call to action
- Resources

## Notes
- Technical requirements
- Commands to execute
- Expected outcomes
```

## Recording Setup

### Software
- **Screen recording:** OBS Studio, QuickTime, ScreenFlow, Camtasia
- **Terminal recording:** Asciinema (for terminal-only demos)
- **Video editing:** DaVinci Resolve, iMovie, Adobe Premiere

### Settings
- **Display resolution:** 1920x1080 or higher
- **Recording area:** Full screen or cropped to relevant area
- **Cursor highlighting:** Enable for better visibility
- **Audio:** Use good quality microphone, quiet environment

### Before Recording
- [ ] Clean desktop and browser
- [ ] Close unnecessary applications
- [ ] Set up demo environment
- [ ] Test microphone and audio levels
- [ ] Prepare script or outline
- [ ] Hide sensitive information (API keys, passwords)

### During Recording
- [ ] Speak clearly and at moderate pace
- [ ] Use cursor highlighting for important clicks
- [ ] Pause 2-3 seconds between major steps
- [ ] Narrate what you're doing
- [ ] Keep mouse movements smooth
- [ ] Allow time for UI to load

### After Recording
- [ ] Review for errors or confusion
- [ ] Add intro/outro (optional)
- [ ] Add background music (optional, low volume)
- [ ] Add captions/subtitles (recommended)
- [ ] Compress and optimize video
- [ ] Test playback on different devices

## External Hosting

**Recommended:** Host videos externally to keep repository size small.

### YouTube
```bash
# Upload to YouTube (unlisted or public)
# Embed in documentation:
```
```markdown
[![Video Title](./thumbnails/demo-thumbnail.png)](https://www.youtube.com/watch?v=VIDEO_ID)
```

### Asciinema (Terminal Recordings)
```bash
# Record terminal session
asciinema rec demo.cast

# Upload to asciinema.org
asciinema upload demo.cast

# Embed in documentation
```
```markdown
[![asciicast](https://asciinema.org/a/RECORDING_ID.svg)](https://asciinema.org/a/RECORDING_ID)
```

## Video Thumbnails

Create custom thumbnails in `thumbnails/` subdirectory:
- **Resolution:** 1280x720
- **Format:** JPG or PNG
- **Content:** Show main topic, include text overlay
- **Branding:** Include KubeRDE logo

## Example Recording Commands

### Using OBS Studio
```bash
# Configure OBS:
# 1. Settings > Output > Recording Format: mp4
# 2. Video Bitrate: 6000 Kbps
# 3. Encoder: x264
# 4. Audio Bitrate: 160 Kbps
```

### Using ffmpeg (screen recording)
```bash
# macOS
ffmpeg -f avfoundation -i "1:0" \
  -c:v h264 -preset medium -crf 23 \
  -c:a aac -b:a 128k \
  demo.mp4

# Linux
ffmpeg -f x11grab -s 1920x1080 -i :0.0 \
  -f pulse -i default \
  -c:v h264 -preset medium -crf 23 \
  -c:a aac -b:a 128k \
  demo.mp4
```

### Compressing existing video
```bash
# Using HandBrake CLI
HandBrakeCLI -i input.mp4 -o output.mp4 \
  --preset="Fast 1080p30" \
  --audio-lang-list eng \
  --subtitle none

# Using ffmpeg
ffmpeg -i input.mp4 \
  -c:v libx264 -preset medium -crf 23 \
  -c:a aac -b:a 128k \
  output.mp4
```

## Terminal Recording (Asciinema)

For command-line tutorials:

```bash
# Install asciinema
brew install asciinema  # macOS
apt install asciinema   # Ubuntu

# Record session
asciinema rec tutorial-ssh.cast

# Perform commands...

# Stop recording (Ctrl+D)

# Preview locally
asciinema play tutorial-ssh.cast

# Upload to asciinema.org
asciinema upload tutorial-ssh.cast
```

## Accessibility

- **Captions:** Add closed captions for all narrated content
- **Transcripts:** Provide written transcripts alongside videos
- **Audio description:** Consider audio descriptions for visual demonstrations

Tools:
- YouTube auto-captions (edit for accuracy)
- Rev.com (professional captioning service)
- Otter.ai (automated transcription)

## Video Checklist

Before publishing a video:

- [ ] Audio is clear and at consistent volume
- [ ] No background noise or distractions
- [ ] Video quality is 1080p minimum
- [ ] All text is readable (check on mobile)
- [ ] Pacing is appropriate (not too fast/slow)
- [ ] Steps are clearly demonstrated
- [ ] No sensitive information visible
- [ ] Intro/outro included (if applicable)
- [ ] Captions/subtitles added
- [ ] Thumbnail created
- [ ] Video optimized for web
- [ ] Uploaded to hosting platform
- [ ] Embedded in documentation

## Resources

- [OBS Studio Guide](https://obsproject.com/wiki/)
- [Asciinema Documentation](https://asciinema.org/docs/)
- [YouTube Creator Academy](https://creatoracademy.youtube.com/)
- [HandBrake Documentation](https://handbrake.fr/docs/)
- [ffmpeg Documentation](https://ffmpeg.org/documentation.html)

# Media Assets

This directory contains media assets for KubeRDE documentation.

## Directory Structure

```
media/
├── screenshots/        # Product screenshots
├── videos/            # Demo and tutorial videos
├── diagrams/          # Architecture and flow diagrams
└── logos/             # Project logos and branding
```

## Screenshots

Place product screenshots in the `screenshots/` directory with descriptive names.

### Required Screenshots

**Main UI Screenshots:**
- `dashboard-overview.png` - Main dashboard view
- `workspace-list.png` - Workspace management page
- `workspace-create.png` - Workspace creation form
- `service-detail.png` - Service detail view
- `user-management.png` - User management interface
- `audit-logs.png` - Audit logs view

**Feature Screenshots:**
- `ssh-connection.png` - SSH connection to workspace
- `jupyter-notebook.png` - Jupyter notebook in action
- `resource-monitoring.png` - Resource usage monitoring
- `template-management.png` - Template management UI

**Mobile/Responsive:**
- `mobile-dashboard.png` - Mobile view of dashboard
- `tablet-workspace.png` - Tablet view of workspace management

### Naming Convention

Use descriptive, lowercase names with hyphens:
- ✅ Good: `workspace-create-form.png`
- ❌ Bad: `Screenshot 2024-01-05.png`

### Specifications

- Format: PNG (preferred) or JPG
- Resolution: At least 1920x1080 for desktop screenshots
- Size: Optimize images (< 500KB per screenshot)
- Include UI chrome: Yes (show browser/app context)
- Annotations: Use for highlighting features

### Tools

Recommended screenshot tools:
- macOS: `Cmd+Shift+4` (built-in), CleanShot X
- Windows: Snipping Tool, ShareX
- Linux: Flameshot, GNOME Screenshot
- Browser: Full Page Screen Capture extensions

## Videos

Place video files in the `videos/` directory.

### Required Videos

**Demo Video:**
- `demo.mp4` - Main product demo (3-5 minutes)
  - Introduction to KubeRDE
  - Key features walkthrough
  - End-to-end workflow demonstration

**Tutorial Videos:**
- `tutorial-01-quick-start.mp4` - Getting started (5 min)
- `tutorial-02-workspace-management.mp4` - Managing workspaces (7 min)
- `tutorial-03-ssh-access.mp4` - SSH access and configuration (5 min)
- `tutorial-04-production-deployment.mp4` - Production deployment (10 min)

### Specifications

- Format: MP4 (H.264 codec)
- Resolution: 1920x1080 (1080p)
- Frame rate: 30 fps
- Bitrate: 5-8 Mbps for good quality
- Audio: AAC codec, 128-192 kbps
- Length: 3-10 minutes per video
- Size: Optimize for web (< 100MB per video)

### Recording Tips

1. **Preparation:**
   - Clean desktop and browser
   - Hide sensitive information
   - Use consistent theme/settings
   - Prepare script or outline

2. **During Recording:**
   - Speak clearly and slowly
   - Use cursor highlighting
   - Keep mouse movements smooth
   - Pause between steps

3. **Post-Production:**
   - Add intro/outro slides
   - Include background music (optional)
   - Add captions/subtitles
   - Include chapter markers

### Recording Tools

- macOS: QuickTime, ScreenFlow, Camtasia
- Windows: OBS Studio, Camtasia, ScreenFlow
- Linux: SimpleScreenRecorder, OBS Studio
- Online: Loom, Screencastify

## Diagrams

Architecture and flow diagrams should be placed in `diagrams/`.

### Required Diagrams

- `architecture-overview.png` - High-level architecture
- `data-flow.png` - Data flow diagram
- `authentication-flow.png` - Authentication sequence
- `agent-connection.png` - Agent connection flow
- `deployment-topology.png` - Deployment topology

### Tools

- Draw.io / diagrams.net (recommended)
- Lucidchart
- Mermaid (code-based, committed as .mmd files)
- Excalidraw

### Specifications

- Format: PNG or SVG (vector preferred)
- Background: Transparent or white
- Style: Consistent colors and fonts
- Labels: Clear and readable
- Export: High resolution (300 DPI for PNG)

## Logos

Project logos and branding assets in `logos/`.

### Files

- `kuberde-logo.png` - Main logo (PNG)
- `kuberde-logo.svg` - Main logo (vector)
- `kuberde-icon.png` - App icon (512x512)
- `kuberde-wordmark.png` - Text logo
- `kuberde-logo-dark.png` - Dark mode version
- `kuberde-logo-light.png` - Light mode version

### Usage Guidelines

- Always maintain aspect ratio
- Use SVG for scalable needs
- Minimum size: 32x32 pixels
- Clear space: 10% of logo width
- Background: Transparent preferred

## Referencing Media in Documentation

### Screenshots

```markdown
![Dashboard Overview](../media/screenshots/dashboard-overview.png)
```

### Videos

```markdown
<!-- Embedded video -->
<video width="100%" controls>
  <source src="../media/videos/demo.mp4" type="video/mp4">
  Your browser does not support the video tag.
</video>

<!-- Or link to external hosting -->
[![Demo Video](../media/screenshots/video-thumbnail.png)](https://www.youtube.com/watch?v=VIDEO_ID)
```

### Diagrams

```markdown
![Architecture Diagram](../media/diagrams/architecture-overview.png)
```

## External Hosting

For large video files, consider hosting on:
- YouTube (public or unlisted)
- Vimeo
- Asciinema (for terminal recordings)
- GitHub Releases (for downloadable files)

Update documentation with external links instead of committing large binaries to the repository.

## File Size Guidelines

To keep the repository size manageable:

- **Screenshots:** < 500KB each (use PNG optimization)
- **Videos:** Host externally, include only thumbnail in repo
- **Diagrams:** < 200KB each (use SVG when possible)
- **Total media:** Try to keep under 10MB total in repository

## Contributing Media

When contributing new media assets:

1. Follow naming conventions
2. Optimize file sizes
3. Add descriptive alt text in documentation
4. Update this README if adding new categories
5. Consider external hosting for large files

## License

All media assets should be created by you or properly licensed. Include attribution if using third-party assets.

## Need Help?

- Screenshot editing: GIMP, Photoshop, Figma
- Video editing: DaVinci Resolve, iMovie, Adobe Premiere
- Diagram creation: Draw.io, Mermaid
- Optimization: TinyPNG, HandBrake, ImageOptim

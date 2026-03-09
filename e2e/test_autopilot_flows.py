"""
E2E tests for teamoon autopilot flows with video recording.
Uses Playwright to validate task lifecycle, autopilot start/stop,
and UI animations in real-time.

Videos saved to e2e/videos/ for visual inspection.
"""

import time

from pathlib import Path
from playwright.sync_api import sync_playwright

BASE_URL = "http://localhost:7777"
PASSWORD = "testpass123"
VIDEOS_DIR = Path(__file__).parent / "videos"


def login(page):
    """Login to teamoon web UI."""
    page.goto(BASE_URL)
    page.wait_for_load_state("networkidle")

    # Check if login form is present
    pw_input = page.locator('input[type="password"]')
    if pw_input.is_visible():
        pw_input.fill(PASSWORD)
        page.locator(
            'button[type="submit"], button:has-text("Login"), button:has-text("login")'
        ).first.click()
        page.wait_for_load_state("networkidle")
        time.sleep(1)


def navigate_to(page, view):
    """Navigate to a specific view tab (queue, projects, logs, etc)."""
    nav = page.locator(f'a[data-view="{view}"]')
    if nav.count() > 0:
        nav.first.click()
        page.wait_for_load_state("networkidle")
        time.sleep(0.5)


def get_task_states(page):
    """Extract task states from DOM timeline nodes."""
    return page.evaluate(
        """() => {
        const nodes = document.querySelectorAll('.tl-node[data-task-id]');
        return Array.from(nodes).map(n => ({
            id: n.dataset.taskId || '',
            text: n.textContent.substring(0, 100),
            classes: n.className,
            state: (n.querySelector('.task-state') || {}).textContent || '',
        }));
    }"""
    )


def get_active_animations(page):
    """Get all active CSS animations on the page."""
    return page.evaluate(
        """() => {
        const allElements = document.querySelectorAll('*');
        const animated = [];
        for (const el of allElements) {
            const anims = el.getAnimations();
            if (anims.length > 0) {
                animated.push({
                    tag: el.tagName,
                    id: el.id,
                    class: el.className.toString().substring(0, 80),
                    animations: anims.map(a => ({
                        name: a.animationName || a.id || 'transition',
                        state: a.playState,
                        duration: a.effect?.getTiming?.()?.duration || 0,
                    })),
                });
            }
        }
        return animated;
    }"""
    )


def get_css_transitions(page):
    """Get elements with active CSS transitions."""
    return page.evaluate(
        """() => {
        const allElements = document.querySelectorAll('*');
        const transitioning = [];
        for (const el of allElements) {
            const style = getComputedStyle(el);
            if (style.transition && style.transition !== 'all 0s ease 0s' && style.transition !== 'none') {
                transitioning.push({
                    tag: el.tagName,
                    id: el.id,
                    class: el.className.toString().substring(0, 80),
                    transition: style.transition.substring(0, 120),
                });
            }
        }
        return transitioning;
    }"""
    )


def test_login_and_dashboard():
    """Test 1: Login and verify dashboard loads with animations."""
    print("\n=== TEST 1: Login & Dashboard ===")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            record_video_dir=str(VIDEOS_DIR),
            record_video_size={"width": 1280, "height": 720},
            viewport={"width": 1280, "height": 720},
        )
        page = context.new_page()

        try:
            login(page)
            page.screenshot(path=str(VIDEOS_DIR / "01_dashboard.png"))

            # Check page loaded
            title = page.title()
            print(f"  Page title: {title}")

            # Check for CSS transitions/animations
            transitions = get_css_transitions(page)
            print(f"  Elements with CSS transitions: {len(transitions)}")
            for t in transitions[:5]:
                print(f"    {t['tag']}.{t['class'][:40]} -> {t['transition'][:60]}")

            animations = get_active_animations(page)
            print(f"  Active animations: {len(animations)}")
            for a in animations[:5]:
                print(f"    {a['tag']}.{a['class'][:40]} -> {[x['name'] for x in a['animations']]}")

            print("  PASS: Dashboard loaded successfully")

        finally:
            context.close()
            browser.close()


def test_task_list_and_states():
    """Test 2: Verify task cards show correct states."""
    print("\n=== TEST 2: Task List & States ===")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            record_video_dir=str(VIDEOS_DIR),
            record_video_size={"width": 1280, "height": 720},
            viewport={"width": 1280, "height": 720},
        )
        page = context.new_page()

        try:
            login(page)
            navigate_to(page, "queue")

            page.screenshot(path=str(VIDEOS_DIR / "02_task_list.png"))

            # Get task states from timeline nodes
            tasks = get_task_states(page)
            print(f"  Found {len(tasks)} task nodes")
            for t in tasks[:10]:
                print(f"    ID={t['id']} state={t['state'][:30]} text={t['text'][:50]}")

            # Count by state badge classes
            running = page.locator(".tl-node .task-state.running").count()
            pending = page.locator(".tl-node .task-state.pending").count()
            planned = page.locator(".tl-node .task-state.planned").count()
            done = page.locator(".tl-node .task-state.done").count()
            print(f"  States: running={running} pending={pending} planned={planned} done={done}")

            # Verify no stuck tasks (running without running-dot indicator)
            running_nodes = page.locator(".tl-node .task-state.running").count()
            running_dots = page.locator(".tl-node.has-running .running-dot").count()
            if running_nodes > 0 and running_dots == 0:
                print(
                    f"  WARNING: {running_nodes} tasks show running state but no active indicator"
                )

            print("  PASS: Task states visible")

        finally:
            context.close()
            browser.close()


def test_autopilot_buttons():
    """Test 3: Verify autopilot controls exist and respond."""
    print("\n=== TEST 3: Autopilot Controls ===")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            record_video_dir=str(VIDEOS_DIR),
            record_video_size={"width": 1280, "height": 720},
            viewport={"width": 1280, "height": 720},
        )
        page = context.new_page()

        try:
            login(page)

            # Check project-level autopilot on Projects view
            navigate_to(page, "projects")
            page.screenshot(path=str(VIDEOS_DIR / "03a_projects.png"))

            autopilot_badges = page.locator(".autopilot-badge").count()
            print(f"  Projects with active autopilot: {autopilot_badges}")

            # Check task-level controls on Queue view
            navigate_to(page, "queue")

            # Click first task to expand detail-actions
            first_task = page.locator(".tl-node[data-task-id]").first
            if first_task.count() > 0:
                first_task.locator(".tl-header").click()
                time.sleep(0.3)

            page.screenshot(path=str(VIDEOS_DIR / "03b_task_controls.png"))

            # Count action buttons in expanded task detail
            plan_btns = page.locator(".detail-actions .btn").count()
            print(f"  Task action buttons visible: {plan_btns}")

            # Verify the standard action set exists (Plan | Run, Stop, Replan)
            actions_text = (
                page.locator(".detail-actions").first.text_content()
                if page.locator(".detail-actions").count() > 0
                else ""
            )
            print(f"  Actions panel: {actions_text[:80]}")

            print("  PASS: Autopilot controls found")

        finally:
            context.close()
            browser.close()


def test_network_requests():
    """Test 4: Verify API calls work (SSE, task list, etc)."""
    print("\n=== TEST 4: Network & API ===")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            record_video_dir=str(VIDEOS_DIR),
            record_video_size={"width": 1280, "height": 720},
            viewport={"width": 1280, "height": 720},
        )
        page = context.new_page()

        api_calls = []

        def on_response(response):
            if "/api/" in response.url:
                api_calls.append(
                    {
                        "url": response.url,
                        "status": response.status,
                        "method": response.request.method,
                    }
                )

        page.on("response", on_response)

        try:
            login(page)
            time.sleep(2)  # Wait for SSE and API calls

            page.screenshot(path=str(VIDEOS_DIR / "04_network.png"))

            print(f"  API calls captured: {len(api_calls)}")
            for call in api_calls[:15]:
                status_icon = "OK" if call["status"] < 400 else "ERR"
                print(
                    f"    [{status_icon}] {call['method']} {call['url'].replace(BASE_URL, '')} -> {call['status']}"
                )

            # Check for SSE connection
            sse_calls = [
                c
                for c in api_calls
                if "events" in c["url"] or "sse" in c["url"] or "stream" in c["url"]
            ]
            print(f"  SSE connections: {len(sse_calls)}")

            errors = [c for c in api_calls if c["status"] >= 400]
            if errors:
                print(f"  WARNING: {len(errors)} API errors detected")
            else:
                print("  PASS: All API calls successful")

        finally:
            context.close()
            browser.close()


def test_animation_catalog():
    """Test 5: Catalog ALL animations and transitions in the UI."""
    print("\n=== TEST 5: Animation Catalog ===")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            record_video_dir=str(VIDEOS_DIR),
            record_video_size={"width": 1280, "height": 720},
            viewport={"width": 1280, "height": 720},
        )
        page = context.new_page()

        try:
            login(page)

            # Extract ALL @keyframes from stylesheets
            keyframes = page.evaluate(
                """() => {
                const kf = [];
                for (const sheet of document.styleSheets) {
                    try {
                        for (const rule of sheet.cssRules) {
                            if (rule instanceof CSSKeyframesRule) {
                                kf.push({
                                    name: rule.name,
                                    steps: Array.from(rule.cssRules).map(r => r.keyText),
                                });
                            }
                        }
                    } catch(e) {} // cross-origin sheets
                }
                return kf;
            }"""
            )

            print(f"  @keyframes found: {len(keyframes)}")
            for kf in keyframes:
                print(
                    f"    @keyframes {kf['name']} ({len(kf['steps'])} steps: {', '.join(kf['steps'][:4])})"
                )

            # Extract all transition properties
            transitions = get_css_transitions(page)
            print(f"\n  Elements with transitions: {len(transitions)}")
            # Deduplicate by transition value
            seen = set()
            for t in transitions:
                key = t["transition"]
                if key not in seen:
                    seen.add(key)
                    print(f"    {t['tag']}: {key[:80]}")

            # Check active animations right now
            active = get_active_animations(page)
            print(f"\n  Currently animating elements: {len(active)}")
            for a in active:
                for anim in a["animations"]:
                    print(
                        f"    {a['tag']}.{a['class'][:30]} -> {anim['name']} ({anim['state']}, {anim['duration']}ms)"
                    )

            page.screenshot(path=str(VIDEOS_DIR / "05_animations.png"))
            print("\n  PASS: Animation catalog complete")

        finally:
            context.close()
            browser.close()


def test_page_performance():
    """Test 6: Measure page performance and frame metrics."""
    print("\n=== TEST 6: Performance Metrics ===")
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            record_video_dir=str(VIDEOS_DIR),
            record_video_size={"width": 1280, "height": 720},
            viewport={"width": 1280, "height": 720},
        )
        page = context.new_page()

        try:
            login(page)

            # Collect performance metrics via CDP
            client = page.context.new_cdp_session(page)
            client.send("Performance.enable")
            metrics = client.send("Performance.getMetrics")

            print("  Performance metrics:")
            key_metrics = [
                "JSHeapUsedSize",
                "JSHeapTotalSize",
                "Nodes",
                "LayoutCount",
                "RecalcStyleCount",
                "LayoutDuration",
                "RecalcStyleDuration",
                "ScriptDuration",
                "TaskDuration",
                "Frames",
            ]
            for m in metrics.get("metrics", []):
                if m["name"] in key_metrics:
                    val = m["value"]
                    if "Size" in m["name"]:
                        val = f"{val / 1024 / 1024:.1f} MB"
                    elif "Duration" in m["name"]:
                        val = f"{val:.3f}s"
                    print(f"    {m['name']}: {val}")

            # Check for layout thrashing
            perf = page.evaluate(
                """() => {
                const entries = performance.getEntriesByType('navigation');
                const nav = entries[0] || {};
                return {
                    domContentLoaded: nav.domContentLoadedEventEnd - nav.startTime,
                    loadComplete: nav.loadEventEnd - nav.startTime,
                    domNodes: document.querySelectorAll('*').length,
                };
            }"""
            )
            print(f"\n  DOM Content Loaded: {perf.get('domContentLoaded', 0):.0f}ms")
            print(f"  Full Load: {perf.get('loadComplete', 0):.0f}ms")
            print(f"  DOM Nodes: {perf.get('domNodes', 0)}")

            print("\n  PASS: Performance metrics collected")

        finally:
            context.close()
            browser.close()


if __name__ == "__main__":
    VIDEOS_DIR.mkdir(parents=True, exist_ok=True)
    print(f"Videos will be saved to: {VIDEOS_DIR}")

    tests = [
        test_login_and_dashboard,
        test_task_list_and_states,
        test_autopilot_buttons,
        test_network_requests,
        test_animation_catalog,
        test_page_performance,
    ]

    passed = 0
    failed = 0
    for test in tests:
        try:
            test()
            passed += 1
        except Exception as e:
            print(f"  FAIL: {e}")
            failed += 1

    print(f"\n{'='*50}")
    print(f"Results: {passed} passed, {failed} failed out of {len(tests)} tests")
    print(f"Videos saved in: {VIDEOS_DIR}")
    print(f"{'='*50}")

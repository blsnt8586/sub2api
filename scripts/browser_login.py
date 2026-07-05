#!/usr/bin/env python3
"""
browser_login.py — 通过 undetected-chromedriver 绕过 Cloudflare Turnstile 登录
供 sub2api Go 后端通过 exec.Command 调用

用法:
    python3 browser_login.py --url URL --email EMAIL --password PASS --output json

输出 (stdout):
    {"auth_token": "eyJ...", "refresh_token": "rt_..."}
    或
    {"error": "失败原因"}
"""

import argparse
import json
import logging
import os
import signal
import subprocess
import sys
import time

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(message)s",
    stream=sys.stderr,  # 日志走 stderr，JSON 结果走 stdout
)
log = logging.getLogger(__name__)

XVFB_DISPLAY = ":99"


def _need_xvfb() -> bool:
    """无 DISPLAY 环境变量时需要启 Xvfb（生产服务器场景）"""
    return not os.environ.get("DISPLAY")


def _start_xvfb() -> "subprocess.Popen | None":
    if os.path.exists(f"/tmp/.X{XVFB_DISPLAY[1:]}-lock"):
        log.info("Xvfb %s 已在运行", XVFB_DISPLAY)
        os.environ["DISPLAY"] = XVFB_DISPLAY
        return None
    try:
        proc = subprocess.Popen(
            ["Xvfb", XVFB_DISPLAY, "-screen", "0", "1280x800x24", "-ac"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        time.sleep(1.5)
        os.environ["DISPLAY"] = XVFB_DISPLAY
        log.info("Xvfb %s 已启动 (pid=%d)", XVFB_DISPLAY, proc.pid)
        return proc
    except FileNotFoundError:
        log.warning("Xvfb 未安装，回退到屏幕外模式")
        return None


def login(base_url: str, email: str, password: str) -> dict:
    import undetected_chromedriver as uc
    from selenium.webdriver.common.by import By
    from selenium.webdriver.support import expected_conditions as EC
    from selenium.webdriver.support.ui import WebDriverWait
    from selenium.common.exceptions import TimeoutException

    xvfb_proc = None
    if _need_xvfb():
        xvfb_proc = _start_xvfb()
        use_offscreen = xvfb_proc is None  # Xvfb 未能启动则用屏幕外模式
    else:
        use_offscreen = True  # 有 DISPLAY（WSL2/桌面）用屏幕外模式

    options = uc.ChromeOptions()
    options.add_argument("--password-store=basic")
    options.add_argument("--no-first-run")
    options.add_argument("--no-sandbox")
    options.add_argument("--disable-dev-shm-usage")

    # 关键修复：用 SwiftShader 软件渲染代替 --disable-gpu
    # --disable-gpu 会让 WebGL 完全不可用，导致 Turnstile JS 初始化失败、不创建 iframe
    # SwiftShader 提供软件模拟的 WebGL，让 Turnstile 正常加载并自动解决
    options.add_argument("--use-gl=swiftshader")
    options.add_argument("--use-angle=swiftshader-webgl")
    options.add_argument("--enable-webgl")
    options.add_argument("--ignore-gpu-blocklist")  # 覆盖服务器环境的 GPU 黑名单
    options.add_argument("--disable-software-rasterizer")  # 防止回退到完全无 WebGL 模式

    if use_offscreen:
        options.add_argument("--window-position=-32000,-32000")
        options.add_argument("--window-size=1280,800")

    # Chrome 版本：优先读环境变量 SUB2API_CHROME_VERSION，默认 150（服务器 Chromium 150）
    chrome_version = int(os.environ.get("SUB2API_CHROME_VERSION", "150"))
    driver = uc.Chrome(options=options, version_main=chrome_version)
    try:
        # ----------------------------------------------------------------
        # 注入 WebGL 指纹伪造：把 SwiftShader 伪装成真实 Intel GPU
        # Turnstile 通过 getParameter(37445/37446) 检测渲染器
        # SwiftShader 返回 "Google SwiftShader" → 触发 checkbox 模式
        # 伪造成 Intel GPU → Turnstile 自动通过（与 WSL2 行为一致）
        # ----------------------------------------------------------------
        driver.execute_cdp_cmd("Page.addScriptToEvaluateOnNewDocument", {
            "source": """
            (function() {
                // 伪造 WebGL1 指纹
                const origGetParam = WebGLRenderingContext.prototype.getParameter;
                WebGLRenderingContext.prototype.getParameter = function(parameter) {
                    if (parameter === 37445) return 'Intel Inc.';
                    if (parameter === 37446) return 'Intel(R) Iris(TM) Plus Graphics 640';
                    return origGetParam.apply(this, arguments);
                };
                // 伪造 WebGL2 指纹
                if (typeof WebGL2RenderingContext !== 'undefined') {
                    const orig2 = WebGL2RenderingContext.prototype.getParameter;
                    WebGL2RenderingContext.prototype.getParameter = function(parameter) {
                        if (parameter === 37445) return 'Intel Inc.';
                        if (parameter === 37446) return 'Intel(R) Iris(TM) Plus Graphics 640';
                        return orig2.apply(this, arguments);
                    };
                }
                // 伪造 navigator.hardwareConcurrency (8核)
                Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 8 });
                // 伪造 navigator.platform
                Object.defineProperty(navigator, 'platform', { get: () => 'Linux x86_64' });
            })();
            """
        })
        log.info("WebGL 指纹伪造已注入")

        log.info("访问 %s ...", base_url)
        driver.get(base_url)
        time.sleep(3)

        # 关闭免责声明（直接 JS 触发，不依赖点击）
        try:
            driver.execute_script("""
                const btn = document.querySelector('button[data-disclaimer-confirm]');
                if (btn) btn.click();
            """)
            time.sleep(2)
        except Exception:
            pass

        # 截图：免责声明关闭后
        driver.save_screenshot("/tmp/step1_after_disclaimer.png")
        log.info("截图已保存: /tmp/step1_after_disclaimer.png")

        # 打开登录弹窗（JS 触发，绕过所有遮挡）
        driver.execute_script("""
            const btn = document.querySelector('button[data-dialog-open="login"]');
            if (btn) btn.click();
        """)
        time.sleep(2)

        # 截图：点击 login 按钮后
        driver.save_screenshot("/tmp/step2_after_login_click.png")
        log.info("截图已保存: /tmp/step2_after_login_click.png")
        log.info("当前 URL: %s", driver.current_url)
        log.info("页面标题: %s", driver.title)

        # 填写表单（弹窗已打开，send_keys 直接写入）
        email_input = WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "input[name='email']"))
        )
        email_input.clear()
        email_input.send_keys(email)

        pwd_input = driver.find_element(By.CSS_SELECTOR, "input[name='password']")
        pwd_input.clear()
        pwd_input.send_keys(password)

        log.info("登录表单填写完毕，等待 Turnstile 加载...")

        # 触发鼠标移动，帮助 Turnstile widget 加载
        from selenium.webdriver.common.action_chains import ActionChains
        try:
            actions = ActionChains(driver)
            actions.move_to_element(email_input).perform()
            time.sleep(0.5)
            actions.move_to_element(pwd_input).perform()
            time.sleep(0.5)
        except Exception:
            pass

        # ----------------------------------------------------------------
        # Turnstile 处理：等待 widget 加载，然后找 checkbox 点击
        # 注意：Turnstile 在某些配置下不用 iframe，直接内嵌到主 DOM
        # ----------------------------------------------------------------
        log.info("等待 Turnstile widget 加载（5秒）...")
        time.sleep(5)

        # 诊断：打印页面所有 input 元素
        dom_info = driver.execute_script("""
            const inputs = Array.from(document.querySelectorAll('input'));
            return inputs.map(el => ({
                type: el.type,
                name: el.name,
                id: el.id,
                className: el.className.substring(0, 60),
                visible: el.offsetParent !== null,
                rect: (function(){
                    var r = el.getBoundingClientRect();
                    return {x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height)};
                })()
            }));
        """)
        log.info("页面 input 元素 (%d 个):", len(dom_info))
        for el in dom_info:
            log.info("  type=%-10s name=%-30s id=%-20s visible=%s rect=%s",
                     el.get('type',''), el.get('name',''), el.get('id',''),
                     el.get('visible'), el.get('rect'))

        # 也打印 iframe 信息
        all_iframes = driver.find_elements(By.TAG_NAME, "iframe")
        log.info("iframe 数量: %d", len(all_iframes))
        for i, f in enumerate(all_iframes):
            try:
                log.info("  iframe[%d] src=%s id=%s name=%s",
                         i, f.get_attribute("src"), f.get_attribute("id"), f.get_attribute("name"))
            except Exception:
                pass

        # turnstile widget IDs
        widget_info = driver.execute_script("""
            if (!window.turnstile) return null;
            try {
                // 尝试获取所有 widget
                const widgets = window.turnstile._widgets || window.turnstile.widgets;
                if (widgets) return JSON.stringify(Object.keys(widgets));
            } catch(e) {}
            return 'turnstile-exists-no-widgets-api';
        """)
        log.info("Turnstile widget 信息: %s", widget_info)

        clicked = False

        # 方法A: 直接找主 DOM 里的 checkbox（Turnstile 非 iframe 嵌入）
        if not clicked:
            try:
                cb_result = driver.execute_script("""
                    // 找所有 checkbox
                    const cbs = Array.from(document.querySelectorAll('input[type="checkbox"]'));
                    for (const cb of cbs) {
                        // 点击所有可见的 checkbox
                        if (cb.offsetParent !== null || cb.style.display !== 'none') {
                            cb.click();
                            const r = cb.getBoundingClientRect();
                            return 'clicked-checkbox: ' + cb.className + ' at ' + JSON.stringify({x:r.x,y:r.y});
                        }
                    }
                    return 'no-checkbox-found (total=' + cbs.length + ')';
                """)
                log.info("方法A 结果: %s", cb_result)
                if cb_result and 'clicked-checkbox' in cb_result:
                    clicked = True
            except Exception as e:
                log.warning("方法A 失败: %s", e)

        # 方法B: label 文字匹配 "human" / "verify"，点击对应 checkbox
        if not clicked:
            try:
                cb_result = driver.execute_script("""
                    const labels = Array.from(document.querySelectorAll('label, span, div'));
                    for (const el of labels) {
                        if (el.textContent && /verify|human/i.test(el.textContent.trim())) {
                            const cb = el.querySelector('input[type="checkbox"]')
                                    || el.closest('label')?.querySelector('input[type="checkbox"]');
                            if (cb) { cb.click(); return 'label-click: ' + el.textContent.trim().substring(0,40); }
                            el.click();
                            return 'el-click: ' + el.tagName + ' ' + el.textContent.trim().substring(0,40);
                        }
                    }
                    return null;
                """)
                if cb_result:
                    log.info("✅ 方法B: %s", cb_result)
                    clicked = True
            except Exception as e:
                log.warning("方法B 失败: %s", e)

        # 方法C: ActionChains 按坐标点击 checkbox 区域（从截图看大约在表单中部）
        if not clicked:
            try:
                # 找 "Human verification" 区域
                cb_rect = driver.execute_script("""
                    const all = Array.from(document.querySelectorAll('*'));
                    for (const el of all) {
                        if (el.children.length === 0 && el.textContent.trim() === 'Verify you are human') {
                            const r = el.getBoundingClientRect();
                            return {x: r.x, y: r.y, w: r.width, h: r.height};
                        }
                    }
                    return null;
                """)
                if cb_rect:
                    log.info("找到 'Verify you are human' 元素位置: %s", cb_rect)
                    # checkbox 在文字左侧约 20px
                    click_x = cb_rect['x'] - 20
                    click_y = cb_rect['y'] + cb_rect['h'] / 2
                    ActionChains(driver).move_by_offset(int(click_x), int(click_y)).click().perform()
                    log.info("✅ 方法C: ActionChains 坐标点击 (%d, %d)", int(click_x), int(click_y))
                    clicked = True
            except Exception as e:
                log.warning("方法C 失败: %s", e)

        # 方法D: window.turnstile 注入 token（绕过 UI，直接调用回调）
        if not clicked:
            try:
                inject_result = driver.execute_script("""
                    if (!window.turnstile) return 'no-turnstile';
                    // 找所有 cf-turnstile-response hidden input，直接触发
                    const hidden = document.querySelector('input[name="cf-turnstile-response"]');
                    if (hidden) {
                        // 找绑定在 widget 上的 callback
                        const containers = document.querySelectorAll('[data-sitekey]');
                        for (const c of containers) {
                            const cb = c.getAttribute('data-callback');
                            if (cb && window[cb]) {
                                window[cb]('DUMMY_TOKEN_BYPASS');
                                return 'callback-injected: ' + cb;
                            }
                        }
                    }
                    return 'no-callback-found';
                """)
                log.info("方法D 结果: %s", inject_result)
            except Exception as e:
                log.warning("方法D 失败: %s", e)

        if not clicked:
            log.info("⚠️ 所有点击方法均未成功，等待 Turnstile 自动验证...")
        else:
            time.sleep(3)

        # 截图看 Turnstile 状态
        driver.save_screenshot("/tmp/step3_turnstile.png")
        log.info("截图已保存: /tmp/step3_turnstile.png")

        # 等待 Turnstile 自动完成（最多 60 秒）
        for i in range(60):
            token = driver.execute_script(
                "return document.querySelector('[name=\"cf-turnstile-response\"]')?.value || ''"
            )
            if token and len(token) > 10:
                log.info("✅ Turnstile 验证通过（第%d秒），token前缀: %s...", i + 1, token[:20])
                break
            if i % 10 == 9:
                log.info("  仍在等待 Turnstile... (%d秒)", i + 1)
                # 每10秒重新检查 iframe
                cur_iframes = driver.find_elements(By.TAG_NAME, "iframe")
                log.info("  当前 iframe 数量: %d", len(cur_iframes))
            time.sleep(1)
        else:
            # 超时截图
            driver.save_screenshot("/tmp/step4_turnstile_timeout.png")
            log.info("截图已保存: /tmp/step4_turnstile_timeout.png")
            # 输出诊断信息
            try:
                diag = driver.execute_script("""
                    return {
                        iframes: document.querySelectorAll('iframe').length,
                        cfTurnstile: document.querySelectorAll('.cf-turnstile, [data-sitekey]').length,
                        webgl: !!document.createElement('canvas').getContext('webgl'),
                        webgl2: !!document.createElement('canvas').getContext('webgl2'),
                    };
                """)
                log.info("诊断信息: %s", diag)
            except Exception:
                pass
            return {"error": "Turnstile 验证超时（60秒）"}

        # 提交
        driver.find_element(By.CSS_SELECTOR, "form[data-auth-form='login'] button[type='submit']").click()
        time.sleep(8)

        current_url = driver.current_url
        if "/dashboard" not in current_url and "/console" not in current_url:
            try:
                msg = driver.find_element(By.CSS_SELECTOR, "[data-auth-message]")
                if msg.is_displayed() and msg.text.strip():
                    return {"error": f"登录失败: {msg.text.strip()}"}
            except Exception:
                pass
            return {"error": f"登录后未跳转到控制台，当前 URL: {current_url}"}

        ls = driver.execute_script("return window.localStorage;")
        auth_token = ls.get("auth_token", "")
        refresh_token = ls.get("refresh_token", "")

        if not auth_token:
            return {"error": "登录成功但未获取到 auth_token"}

        log.info("登录成功，token 已获取")
        return {"auth_token": auth_token, "refresh_token": refresh_token}

    finally:
        driver.quit()
        if xvfb_proc:
            xvfb_proc.terminate()
            log.info("Xvfb 已关闭")


def main():
    parser = argparse.ArgumentParser(description="Browser login for Turnstile-protected platforms")
    parser.add_argument("--url", required=True, help="平台基础 URL")
    parser.add_argument("--email", required=True, help="登录邮箱")
    parser.add_argument("--password", required=True, help="登录密码")
    parser.add_argument("--output", default="json", choices=["json", "text"], help="输出格式")
    args = parser.parse_args()

    result = login(args.url, args.email, args.password)

    # 输出 JSON 到 stdout（Go 端解析）
    print(json.dumps(result, ensure_ascii=False))
    sys.exit(0 if "auth_token" in result else 1)


if __name__ == "__main__":
    # 防止子进程成为僵尸进程
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    main()

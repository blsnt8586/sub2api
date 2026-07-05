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
    stream=sys.stderr,
)
log = logging.getLogger(__name__)

XVFB_DISPLAY = ":99"


def _need_xvfb() -> bool:
    return not os.environ.get("DISPLAY")


def _start_xvfb() -> "subprocess.Popen | None":
    if os.path.exists(f"/tmp/.X{XVFB_DISPLAY[1:]}-lock"):
        log.info("Xvfb %s 已在运行", XVFB_DISPLAY)
        os.environ["DISPLAY"] = XVFB_DISPLAY
        return None
    try:
        proc = subprocess.Popen(
            ["Xvfb", XVFB_DISPLAY, "-screen", "0", "1280x800x24", "-ac", "+extension", "GLX"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        time.sleep(1.5)
        os.environ["DISPLAY"] = XVFB_DISPLAY
        log.info("Xvfb %s 已启动 (pid=%d)", XVFB_DISPLAY, proc.pid)
        return proc
    except FileNotFoundError:
        log.warning("Xvfb 未安装")
        return None


def login(base_url: str, email: str, password: str) -> dict:
    import undetected_chromedriver as uc
    from selenium.webdriver.common.by import By
    from selenium.webdriver.support import expected_conditions as EC
    from selenium.webdriver.support.ui import WebDriverWait
    from selenium.webdriver.common.action_chains import ActionChains

    xvfb_proc = None
    is_server = _need_xvfb()
    if is_server:
        xvfb_proc = _start_xvfb()

    options = uc.ChromeOptions()
    options.add_argument("--no-sandbox")
    options.add_argument("--disable-dev-shm-usage")
    options.add_argument("--password-store=basic")
    options.add_argument("--no-first-run")
    options.add_argument("--window-size=1280,800")

    # 服务器：窗口移出屏幕（Xvfb 有 DISPLAY 但无需可见）
    # 本地 WSL2：同样移出屏幕，静默运行
    options.add_argument("--window-position=-32000,-32000")

    # 不添加任何 GPU 相关 flag
    # --disable-gpu 会让 WebGL 完全失效，Turnstile 检测到后强制显示 checkbox
    # 不加任何 flag 时：
    #   WSL2 → 使用真实 GPU/ANGLE，Turnstile 自动通过
    #   服务器 → Chrome 尝试 EGL/Mesa，比 SwiftShader 更真实

    chrome_version = int(os.environ.get("SUB2API_CHROME_VERSION", "150"))
    driver = uc.Chrome(options=options, version_main=chrome_version)
    try:
        log.info("访问 %s ...", base_url)
        driver.get(base_url)
        time.sleep(3)

        # 关闭免责声明
        try:
            driver.execute_script("""
                const btn = document.querySelector('button[data-disclaimer-confirm]');
                if (btn) btn.click();
            """)
            time.sleep(2)
        except Exception:
            pass

        driver.save_screenshot("/tmp/step1_after_disclaimer.png")
        log.info("截图已保存: /tmp/step1_after_disclaimer.png")

        # 打开登录弹窗
        driver.execute_script("""
            const btn = document.querySelector('button[data-dialog-open="login"]');
            if (btn) btn.click();
        """)
        time.sleep(2)

        driver.save_screenshot("/tmp/step2_after_login_click.png")
        log.info("截图已保存: /tmp/step2_after_login_click.png")

        # 填写表单
        email_input = WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "input[name='email']"))
        )
        email_input.clear()
        email_input.send_keys(email)

        pwd_input = driver.find_element(By.CSS_SELECTOR, "input[name='password']")
        pwd_input.clear()
        pwd_input.send_keys(password)
        log.info("表单填写完毕，等待 Turnstile 加载...")

        # 鼠标移动触发 widget 初始化
        try:
            ActionChains(driver).move_to_element(email_input).perform()
            time.sleep(0.3)
            ActionChains(driver).move_to_element(pwd_input).perform()
            time.sleep(0.5)
        except Exception:
            pass

        # 等待 3 秒让 Turnstile 加载
        time.sleep(3)

        # 检查 WebGL 实际渲染器（诊断用）
        webgl_info = driver.execute_script("""
            try {
                const c = document.createElement('canvas');
                const gl = c.getContext('webgl') || c.getContext('experimental-webgl');
                if (!gl) return 'no-webgl';
                const ext = gl.getExtension('WEBGL_debug_renderer_info');
                if (!ext) return 'no-debug-ext';
                return {
                    vendor: gl.getParameter(ext.UNMASKED_VENDOR_WEBGL),
                    renderer: gl.getParameter(ext.UNMASKED_RENDERER_WEBGL),
                };
            } catch(e) { return 'error:' + e.message; }
        """)
        log.info("WebGL 渲染器: %s", webgl_info)

        driver.save_screenshot("/tmp/step3_turnstile.png")
        log.info("截图已保存: /tmp/step3_turnstile.png")

        # 等待 Turnstile token（最多 60 秒）
        for i in range(60):
            token = driver.execute_script(
                "return document.querySelector('[name=\"cf-turnstile-response\"]')?.value || ''"
            )
            if token and len(token) > 10:
                log.info("✅ Turnstile 通过（第%d秒），token: %s...", i + 1, token[:20])
                break

            # 第5秒时尝试点击 checkbox（如果出现了）
            if i == 5:
                try:
                    cb_info = driver.execute_script("""
                        const cbs = document.querySelectorAll('input[type="checkbox"]');
                        if (cbs.length > 0) {
                            cbs[0].click();
                            return 'clicked-' + cbs.length + '-checkboxes';
                        }
                        return 'no-checkbox';
                    """)
                    log.info("Checkbox 状态: %s", cb_info)
                except Exception:
                    pass

            if i % 10 == 9:
                diag = driver.execute_script("""
                    return {
                        hasWidget: !!window.turnstile,
                        tokenLen: (document.querySelector('[name="cf-turnstile-response"]')?.value||'').length,
                        iframes: document.querySelectorAll('iframe').length,
                    };
                """)
                log.info("  等待 Turnstile (%ds): %s", i + 1, diag)
            time.sleep(1)
        else:
            driver.save_screenshot("/tmp/step4_turnstile_timeout.png")
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
            return {"error": f"登录后未跳转控制台，当前: {current_url}"}

        ls = driver.execute_script("return window.localStorage;")
        auth_token = ls.get("auth_token", "")
        refresh_token = ls.get("refresh_token", "")

        if not auth_token:
            return {"error": "登录成功但未获取到 auth_token"}

        log.info("✅ 登录成功，token 已获取")
        return {"auth_token": auth_token, "refresh_token": refresh_token}

    finally:
        driver.quit()
        if xvfb_proc:
            xvfb_proc.terminate()
            log.info("Xvfb 已关闭")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--email", required=True)
    parser.add_argument("--password", required=True)
    parser.add_argument("--output", default="json", choices=["json", "text"])
    args = parser.parse_args()

    result = login(args.url, args.email, args.password)
    print(json.dumps(result, ensure_ascii=False))
    sys.exit(0 if "auth_token" in result else 1)


if __name__ == "__main__":
    signal.signal(signal.SIGCHLD, signal.SIG_DFL)
    main()

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
    options.add_argument("--disable-gpu")
    if use_offscreen:
        options.add_argument("--window-position=-32000,-32000")
        options.add_argument("--window-size=1280,800")

    # Chrome 版本：优先读环境变量 SUB2API_CHROME_VERSION，默认 150（服务器 Chromium 150）
    chrome_version = int(os.environ.get("SUB2API_CHROME_VERSION", "150"))
    driver = uc.Chrome(options=options, version_main=chrome_version)
    try:
        log.info("访问 %s ...", base_url)
        driver.get(base_url)
        time.sleep(3)

        # 关闭免责声明（等待背景层完全消失后再继续）
        try:
            btn = WebDriverWait(driver, 5).until(
                EC.element_to_be_clickable((By.CSS_SELECTOR, "button[data-disclaimer-confirm]"))
            )
            btn.click()
            # 等待 disclaimer backdrop 消失
            WebDriverWait(driver, 5).until(
                EC.invisibility_of_element_located((By.CSS_SELECTOR, ".disclaimer-backdrop"))
            )
            time.sleep(0.5)
        except Exception:
            pass

        # 打开登录弹窗（用 JS click 绕过偶发遮挡）
        login_btn = WebDriverWait(driver, 10).until(
            EC.presence_of_element_located((By.CSS_SELECTOR, "button[data-dialog-open='login']"))
        )
        driver.execute_script("arguments[0].click();", login_btn)
        time.sleep(2)

        # 填写表单
        WebDriverWait(driver, 10).until(
            EC.element_to_be_clickable((By.CSS_SELECTOR, "input[name='email']"))
        ).send_keys(email)
        driver.find_element(By.CSS_SELECTOR, "input[name='password']").send_keys(password)
        log.info("登录表单填写完毕，等待 Turnstile 验证...")

        # 等待 Turnstile 自动完成
        for i in range(60):
            token = driver.execute_script(
                "return document.querySelector('[name=\"cf-turnstile-response\"]')?.value || ''"
            )
            if token and len(token) > 10:
                log.info("Turnstile 验证通过（第%d秒）", i + 1)
                break
            time.sleep(1)
        else:
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

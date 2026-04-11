import { test, expect } from '@playwright/test';

test.describe('認証フローのE2Eテスト', () => {
  test('ログイン・パスワード変更・ログアウトの一連の操作ができること', async ({ page }) => {
    // 1. ログイン画面の表示確認
    await page.goto('/login');
    await expect(page).toHaveTitle(/ログイン - サンプルアプリ/);

    // 2. ログイン実行
    await page.fill('#email', 'admin@example.com');
    await page.fill('#password', 'initial_password');
    await page.click('button[type="submit"]');

    // 3. ホーム画面への遷移確認
    await expect(page).toHaveURL('/');
    await expect(page.locator('h1')).toHaveText(/Hello, 管理者/);

    // 4. パスワード変更画面への遷移
    await page.click('text=パスワード変更');
    await expect(page).toHaveURL('/password_change');
    await expect(page.locator('h2')).toHaveText('パスワード変更');

    // 5. パスワード変更実行
    await page.fill('#current_password', 'initial_password');
    await page.fill('#new_password', 'new_secret_password');
    await page.fill('#confirm_password', 'new_secret_password');
    await page.click('button[type="submit"]');

    // 成功メッセージの確認
    await expect(page.getByText('パスワードを更新しました。')).toBeVisible();

    // 6. ログアウト
    await page.click('text=ログアウト');
    await expect(page).toHaveURL('/login');

    // 7. 旧パスワードでログインできないことを確認
    await page.fill('#email', 'admin@example.com');
    await page.fill('#password', 'initial_password');
    await page.click('button[type="submit"]');
    await expect(page.getByText('メールアドレスまたはパスワードが正しくありません。')).toBeVisible();

    // 8. 新パスワードでログインできることを確認
    await page.fill('#email', 'admin@example.com');
    await page.fill('#password', 'new_secret_password');
    await page.click('button[type="submit"]');
    await expect(page).toHaveURL('/');

    // 9. 他のテストへの影響を防ぐため、パスワードを元に戻す
    await page.click('text=パスワード変更');
    await page.fill('#current_password', 'new_secret_password');
    await page.fill('#new_password', 'initial_password');
    await page.fill('#confirm_password', 'initial_password');
    await page.click('button[type="submit"]');
    await expect(page.getByText('パスワードを更新しました。')).toBeVisible();
  });
});

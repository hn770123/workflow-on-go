import { test, expect } from '@playwright/test';

test.describe('Hello World アプリケーションのE2Eテスト (認証済み)', () => {
  test.beforeEach(async ({ page }) => {
    // ログイン処理
    await page.goto('/login');
    await page.fill('#email', 'admin@example.com');
    await page.fill('#password', 'initial_password');
    await page.click('button[type="submit"]');

    // ホーム画面に遷移したことを確認
    await expect(page).toHaveURL('/');
  });

  test('ページのタイトルが正しいこと', async ({ page }) => {
    // ページのタイトルを確認 (home.html の定義に合わせる)
    await expect(page).toHaveTitle(/ホーム - サンプルアプリ/);
  });

  test('「Hello, 管理者」の見出しが存在すること', async ({ page }) => {
    // h1タグの中に「Hello, 管理者」というテキストが存在するか確認 (home.html)
    const heading = page.locator('h1');
    await expect(heading).toHaveText(/Hello, 管理者/);
  });

  test('カウンターボタンをクリックすると数値がインクリメントされること', async ({ page }) => {
    // 初期状態のカウンター値「0」を確認 (home.html)
    // spanタグのテキストが「Alpine.js カウンター: 0」の一部であることを考慮
    const counterText = page.locator('div[x-data]').getByText(/Alpine.js カウンター:/);
    await expect(counterText).toContainText('0');

    // 「カウントアップ」ボタンをクリック
    const button = page.getByRole('button', { name: 'カウントアップ' });
    await button.click();

    // クリック後のカウンター値が「1」になっているか確認
    await expect(counterText).toContainText('1');

    // もう一度クリックして「2」になるか確認
    await button.click();
    await expect(counterText).toContainText('2');
  });
});

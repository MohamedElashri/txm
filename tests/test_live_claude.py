"""No authentication or provider processes in these controller regressions."""
import json
from pathlib import Path
import unittest

from live_claude_checks import claude_argv, ui_result


class ClaudeControllerChecks(unittest.TestCase):
    def test_screen_evidence_excludes_raw_content_and_echoed_prompts(self):
        result = ui_result('PRIVATE_PROVIDER_TEXT\n❯ TXM_CLAUDE_ONE\nDo you want to proceed?')
        self.assertTrue(result['permission_prompt'])
        self.assertEqual(result['fixed_replies'], [])
        self.assertNotIn('PRIVATE_PROVIDER_TEXT', json.dumps(result))
        self.assertEqual(ui_result('⏺ TXM_CLAUDE_ONE')['fixed_replies'], ['TXM_CLAUDE_ONE'])
        self.assertEqual(ui_result('● TXM_CLAUDE_ONE')['fixed_replies'], ['TXM_CLAUDE_ONE'])

    def test_resume_preserves_explicit_isolation_without_bypass(self):
        argv = claude_argv(Path('/fixture/claude'), Path('/fixture/hooks.json'), 'thread')
        self.assertEqual(argv[-2:], ['--resume','thread'])
        self.assertIn('--restricted', argv)
        self.assertEqual(argv[argv.index('--tools')+1], 'Read,Bash')
        self.assertEqual(argv[argv.index('--setting-sources')+1], '')
        self.assertEqual(argv[argv.index('--permission-mode')+1], 'default')
        self.assertFalse(any('bypass' in x or 'skip-permission' in x for x in argv))


if __name__ == '__main__':
    unittest.main()

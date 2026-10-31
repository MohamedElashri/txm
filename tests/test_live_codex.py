"""No-auth regressions for the opt-in live controller and prompt budget."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest

from live_codex_checks import CONTROLLER, PromptBudget, report_output, save


class LiveControllerChecks(unittest.TestCase):
    def test_evidence_retains_ui_checks_but_not_provider_screen_contents(self):
        value=report_output('snapshot',{'screen':['PRIVATE_SYNTHETIC_PROMPT','Conversation interrupted','TXM_CODEX_ONE']})
        self.assertNotIn('PRIVATE_SYNTHETIC_PROMPT',json.dumps(value))
        self.assertTrue(value['interrupted'])
        self.assertEqual(value['fixed_markers'],['TXM_CODEX_ONE'])
        self.assertEqual(value['fixed_replies'],[])  # An echoed prompt is not an answer.
        self.assertEqual(report_output('snapshot',{'screen':['• TXM_CODEX_ONE']})['fixed_replies'],['TXM_CODEX_ONE'])

    def test_prompt_reservation_survives_restart_and_refuses_replay_and_capacity(self):
        with tempfile.TemporaryDirectory(prefix='txm-live-budget-') as tmp:
            path=Path(tmp)/'budget.json'
            PromptBudget(path,2).reserve('one','synthetic one')
            budget=PromptBudget(path,2)
            with self.assertRaises(ValueError):budget.reserve('one','another body')
            budget.reserve('two','synthetic two')
            with self.assertRaises(ValueError):budget.reserve('three','synthetic three')
            rows=json.loads(path.read_text())
            self.assertEqual(len(rows),2)
            self.assertNotIn('synthetic',path.read_text())

    def test_controller_does_not_relaunch_a_seen_id(self):
        with tempfile.TemporaryDirectory(prefix='txm-live-driver-') as tmp:
            root=Path(tmp);(root/'repo').mkdir();(root/'controller.py').write_text(CONTROLLER)
            script='from pathlib import Path;import sys\np=Path(sys.argv[1]);p.write_text(p.read_text()+"x" if p.exists() else "x")'
            request={'id':'first','argv':[sys.executable,'-c',script,str(root/'executions')]}
            proc=subprocess.Popen([sys.executable,str(root/'controller.py'),str(root)])
            try:
                save(root/'launch.json',request)
                deadline=time.monotonic()+5
                while not (root/'exit-first.json').exists() and time.monotonic()<deadline:time.sleep(.02)
                self.assertTrue((root/'exit-first.json').exists())
                # Rewrite identical intent: lost-response retry must not relaunch.
                save(root/'launch.json',request);time.sleep(.1)
                self.assertEqual((root/'executions').read_text(),'x')
                request['id']='second';save(root/'launch.json',request)
                deadline=time.monotonic()+5
                while not (root/'exit-second.json').exists() and time.monotonic()<deadline:time.sleep(.02)
                self.assertTrue((root/'exit-second.json').exists())
                self.assertEqual((root/'executions').read_text(),'xx')
            finally:
                (root/'controller-stop').touch();proc.wait(timeout=5)
            self.assertEqual(proc.returncode,0)


if __name__=='__main__':unittest.main()

import unittest
from run_matrix import check_memory, GIB

class MemoryGuardTests(unittest.TestCase):
    def test_stops_before_memory_exhaustion(self):
        with self.assertRaises(RuntimeError):
            check_memory(39, 5 * GIB, 40, 12 * GIB)

    def test_stops_oversized_resident_model(self):
        with self.assertRaises(RuntimeError):
            check_memory(80, 13 * GIB, 40, 12 * GIB)

    def test_accepts_headroom(self):
        check_memory(80, 5 * GIB, 40, 12 * GIB)


class CleanupTests(unittest.TestCase):
    def test_memory_trip_terminates_child_unloads_and_retains_evidence(self):
        import json
        import tempfile
        from pathlib import Path
        from unittest.mock import MagicMock, patch
        import run_matrix

        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = root / 'results'
            process = MagicMock()
            process.poll.return_value = None
            process.pid = 12345
            resident_calls = 0

            def fake_api(path, body=None):
                nonlocal resident_calls
                if path == 'ps':
                    resident_calls += 1
                    return {'models': [] if resident_calls == 1 else [{'name': 'small:1', 'size': 5 * GIB}]}
                if path == 'tags':
                    return {'models': [{'name': 'small:1', 'size': 4 * GIB}]}
                return {'version': 'test'}

            with patch('sys.argv', ['run_matrix.py', '--model', 'small:1', '--output-dir', str(output)]), \
                 patch.object(run_matrix.Path, 'home', return_value=root), \
                 patch.object(run_matrix, 'api', side_effect=fake_api), \
                 patch.object(run_matrix, 'memory_free', side_effect=[90, 90, 20]), \
                 patch.object(run_matrix.subprocess, 'check_output', return_value='{}'), \
                 patch.object(run_matrix.subprocess, 'Popen', return_value=process), \
                 patch.object(run_matrix.os, 'killpg') as kill, \
                 patch.object(run_matrix.signal, 'signal'), \
                 patch.object(run_matrix, 'unload') as unload:
                with self.assertRaisesRegex(RuntimeError, 'Memory limit reached'):
                    run_matrix.main()
                kill.assert_called_once_with(12345, run_matrix.signal.SIGTERM)
                unload.assert_called_once_with('small:1')
            record = json.loads((output / 'execution.json').read_text())[0]
            self.assertTrue(record['unloaded'])
            self.assertIn('Memory limit reached', record['error'])
            self.assertEqual(record['memory'][0]['free_percent'], 20)

if __name__ == '__main__':
    unittest.main()

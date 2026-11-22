import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import uuid


class TranscriptTests(unittest.TestCase):
    def setUp(self):
        root = Path(__file__).resolve().parents[2] / '.project' / 'transcript-tests'
        root.mkdir(parents=True, exist_ok=True)
        temporary = tempfile.TemporaryDirectory(dir=root)
        self.addCleanup(temporary.cleanup)
        self.home = Path(temporary.name)
        self.config = self.home / 'config'
        self.workspace = str(self.home / 'project')
        self.sid = str(uuid.uuid4())
        self.path = self.config / 'projects' / 'project' / (self.sid + '.jsonl')
        self.path.parent.mkdir(parents=True)
        self.env = {**os.environ, 'CAGE_CLI_STATE_ROOT': str(self.home / 'channels'),
                    'CAGE_CLI_ACCOUNT_ROOT': str(self.home / 'accounts')}

    def invoke(self, **values):
        request = {'cli_id': self.sid, 'config': str(self.config), 'workspace': self.workspace,
                   'offset': 0, 'max_bytes': 113, **values}
        message = {'jsonrpc': '2.0', 'id': 1, 'method': 'tools/call',
                   'params': {'name': 'read_session_history', 'arguments': request}}
        plugin = Path(__file__).resolve().parents[2] / 'plugins/happiest/sessions.py'
        result = subprocess.run([sys.executable, '-X', 'utf8', '-I', str(plugin)],
                                input=json.dumps(message), env=self.env, text=True,
                                encoding='utf-8', capture_output=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stderr[-1000:])
        return json.loads(result.stdout)['result']['structuredContent']

    def test_original_bytes_large_tool_and_incremental_append(self):
        rows = [{'cwd': self.workspace, 'uuid': str(uuid.uuid4()), 'type': 'user', 'message': {'content': '开始'}},
            {'cwd': self.workspace, 'uuid': str(uuid.uuid4()), 'type': 'assistant',
             'message': {'content': [{'type': 'tool_use', 'id': 'tool', 'name': 'Bash',
                                      'input': {'command': '完整命令🙂' * 350000}}]}},
            {'cwd': self.workspace, 'uuid': str(uuid.uuid4()), 'type': 'user',
             'message': {'content': [{'type': 'tool_result', 'tool_use_id': 'tool',
                                      'content': '完整输出🙂' * 100000}]}},
            {'type': 'system', 'subtype': 'compact_boundary', 'compactMetadata': {'trigger': 'auto'}},
            {'type': 'assistant', 'isSidechain': True, 'message': {'content': '分支原文'}}]
        original = ('\n'.join(json.dumps(row, ensure_ascii=False) for row in rows) + '\n').encode()
        self.path.write_bytes(original)
        received = bytearray()
        page = self.invoke(max_bytes=524289)
        while True:
            self.assertEqual(page['cli_id'], self.sid)
            received.extend(base64.b64decode(page['content']))
            if page['eof']:
                break
            page = self.invoke(offset=page['next_offset'], source=page['source'], max_bytes=524289)
        self.assertEqual(bytes(received), original)
        self.assertEqual(self.path.read_bytes(), original)
        extra = b'{"type":"user","message":{"content":"append"}}\n'
        with self.path.open('ab') as stream:
            stream.write(extra)
        after = self.invoke(offset=page['next_offset'], source=page['source'])
        self.assertEqual(base64.b64decode(after['content']), extra)

    def test_changed_source_and_path_boundaries(self):
        self.path.write_text(json.dumps({'cwd': self.workspace, 'message': {'content': 'original'}}) + '\n')
        first = self.invoke(max_bytes=10)
        original = self.path.read_bytes()
        self.path.write_bytes(original.replace(b'original', b'modified'))
        self.assertEqual(self.invoke(offset=10, source=first['source'])['error'], 'source_changed')
        self.path.write_bytes(b'changed')
        self.assertEqual(self.invoke(offset=10, source=first['source'])['error'], 'source_changed')
        self.assertEqual(self.invoke(offset=-1)['error'], 'invalid_transcript_request')
        self.assertEqual(self.invoke(offset=1)['error'], 'source_required')
        self.assertEqual(self.invoke(cli_id='../escape')['error'], 'invalid_transcript_request')
        self.path.write_text(json.dumps({'cwd': self.workspace + '-other'}) + '\n')
        self.assertEqual(self.invoke()['error'], 'transcript_workspace_mismatch')
        self.assertEqual(self.invoke(config=str(self.home / 'another-account'), required=True)['error'], 'transcript_unavailable')

    def test_empty_new_history_and_replaced_identity(self):
        empty = self.invoke()
        self.assertEqual(empty['cli_id'], self.sid)
        self.assertEqual((empty['content'], empty['total_bytes'], empty['eof']), ('', 0, True))
        self.path.write_bytes(b'')
        created = self.invoke()
        self.assertEqual((created['content'], created['total_bytes'], created['eof']), ('', 0, True))
        self.path.write_text(json.dumps({'cwd': self.workspace, 'message': {'content': 'same'}}) + '\n')
        first = self.invoke()
        replacement = self.path.with_suffix('.new')
        replacement.write_bytes(self.path.read_bytes())
        os.replace(replacement, self.path)
        self.assertEqual(self.invoke(source=first['source'])['error'], 'source_changed')
        self.path.unlink()
        self.assertEqual(self.invoke(source=first['source'])['error'], 'source_changed')


if __name__ == '__main__':
    unittest.main()

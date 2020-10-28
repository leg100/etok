package k8s

//func TestPodConnect(t *testing.T) {
//	tests := []struct {
//		name       string
//		err        bool
//		tty        bool
//		assertions func(opts *app.Options)
//		out        string
//	}{
//		{
//			name: "attach",
//			tty:  true,
//			out:  "fake attach",
//		},
//		{
//			name: "logs",
//			out:  "fake logs",
//		},
//	}
//
//	for _, tt := range tests {
//		testutil.Run(t, tt.name, func(t *testutil.T) {
//			out := new(bytes.Buffer)
//			opts, err := app.NewFakeOptsWithClients(out)
//
//			if tt.tty {
//				_, tty, err := pty.Open()
//				require.NoError(t, err)
//				opts.In = tty
//			} else {
//				opts.In = bytes.NewBufferString("not a tty")
//			}
//
//			err = PodConnect(
//				context.Background(),
//				opts,
//				testPod("default", "default"),
//			)
//			assert.NoError(t, err)
//
//			assert.Equal(t, tt.out, out.String())
//		})
//	}
//}
//
//func testPod(namespace, name string) *corev1.Pod {
//	return &corev1.Pod{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      name,
//			Namespace: namespace,
//		},
//	}
//}
//
//type testAttachSucceed struct{}
//
//var _ podhandler.Interface = &testAttachSucceed{}
//
//func (h *testAttachSucceed) Attach(cfg *rest.Config, pod *corev1.Pod, out io.Writer) error {
//	fmt.Fprintln(out, "fake attach")
//	return nil
//}
//
//func (h *testAttachSucceed) GetLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
//	return ioutil.NopCloser(bytes.NewBufferString("fake logs")), nil
//}
//
//type testAttachFail struct{}
//
//var _ podhandler.Interface = &testAttachFail{}
//
//func (h *testAttachFail) Attach(cfg *rest.Config, pod *corev1.Pod, out io.Writer) error {
//	return fmt.Errorf("fake error")
//}
//
//func (h *testAttachFail) GetLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
//	return ioutil.NopCloser(bytes.NewBufferString("fake logs")), nil
//}

import {Composition} from 'remotion';
import {ElephantPromo} from './scenes/ElephantPromo';

export const RemotionRoot = () => {
  return (
    <>
      <Composition
        id="ElephantPromo"
        component={ElephantPromo}
        durationInFrames={600}
        fps={30}
        width={1920}
        height={1080}
      />
    </>
  );
};
